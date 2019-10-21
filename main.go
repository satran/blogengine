package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	c := config{
		Host:         os.Getenv("HOSTNAME"),
		Cert:         os.Getenv("CERT"),
		Key:          os.Getenv("KEY"),
		Site:         os.Getenv("SITE"),
		MetricsToken: os.Getenv("METRICS_TOKEN"),
		UseTLS:       true,
	}
	if c.Site == "" {
		log.Fatal("Specify site directory")
	}
	if c.MetricsToken == "" {
		log.Println("warning: running without metrics token")
	}
	if c.Host == "" {
		c.Host = "localhost:8080"
	}
	if c.Cert == "" && c.Key == "" {
		c.UseTLS = false
	}
	c.PagesDir = filepath.Join(c.Site, "blog")
	c.StaticDir = filepath.Join(c.Site, "static")
	c.AliasFile = filepath.Join(c.Site, "alias.json")
	c.TemplatesDir = filepath.Join(c.Site, "templates")
	if err := run(c); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	Host         string
	Cert         string
	Key          string
	Site         string
	PagesDir     string
	StaticDir    string
	TemplatesDir string
	AliasFile    string
	MetricsToken string
	UseTLS       bool
}

func run(c config) error {
	pages, err := parse(c.TemplatesDir, c.PagesDir)
	if err != nil {
		return fmt.Errorf("parse pages: %w", err)
	}
	// for most purposes sorting it reverse stands best
	sort.Sort(sort.Reverse(Pages(pages)))

	aliases, err := getAliases(c.AliasFile)
	if err != nil {
		return fmt.Errorf("load aliases: %w", err)
	}

	m := http.ServeMux{}
	analyze := analyzer()
	m.Handle("/", analyze(handleIndex(c.TemplatesDir, pages, aliases)))
	m.Handle("/feed.xml", analyze(handleFeed(c.Host, c.UseTLS, pages)))
	m.Handle("/b/", analyze(handlePages(pages)))
	m.Handle("/s/", analyze(http.StripPrefix("/s/", http.FileServer(newFileSystem(c.StaticDir)))))
	m.Handle("/metrics", requiresAuth(c.MetricsToken, promhttp.Handler()))

	srv := &http.Server{
		ReadTimeout:  time.Second,
		WriteTimeout: 2 * time.Second,
		Handler:      &m,
	}
	srv.SetKeepAlivesEnabled(false)
	if c.UseTLS {
		srv.Addr = ":443"
		go http.ListenAndServe(":80", http.HandlerFunc(redirect(c.Host)))
		err = srv.ListenAndServeTLS(c.Cert, c.Key)
	} else {
		srv.Addr = ":8080"
		err = srv.ListenAndServe()
	}
	return err
}

func requiresAuth(token string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunks := strings.Split(r.Header.Get("Authorization"), " ")
		if len(chunks) == 2 && chunks[1] == token {
			handler.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
	})
}

func analyzer() func(http.Handler) http.Handler {
	reqDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Time spent in the service",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	})
	totalReqs := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests",
			Help: "HTTP Requests",
		},
		[]string{"code", "method", "path"},
	)
	prometheus.DefaultRegisterer.MustRegister(reqDuration, totalReqs)

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			path := r.URL.Path
			wr := &ResponseWriter{ResponseWriter: w}
			h.ServeHTTP(wr, r)
			since := time.Since(start)
			statusCode := strconv.Itoa(wr.StatusCode())
			reqDuration.Observe(since.Seconds())
			totalReqs.WithLabelValues(statusCode, strings.ToUpper(r.Method), path).Inc()
			log.Println(since, wr.StatusCode(), r.Method, r.URL)
		})
	}
}

func handleIndex(templatesDir string, pages []*Page, aliases map[string]string) http.Handler {
	indexTmpl := template.Must(template.ParseFiles(filepath.Join(templatesDir, "index.html")))
	wr := &bytes.Buffer{}
	if err := indexTmpl.Execute(wr, map[string]interface{}{"Pages": pages}); err != nil {
		panic(err)
	}
	index := wr.Bytes()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write(index)
			return
		}

		alias, ok := aliases[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(http.StatusText(http.StatusNotFound)))
			return
		}

		if len(r.URL.RawQuery) > 0 {
			alias += "?" + r.URL.RawQuery
		}
		log.Printf("redirect to: %s", alias)
		http.Redirect(w, r, alias, http.StatusTemporaryRedirect)
		return
	})
}

func handleFeed(hostname string, tls bool, pages []*Page) http.Handler {
	feed := &feeds.Feed{
		Title:       "satran's blog",
		Link:        &feeds.Link{Href: "https://satran.in"},
		Description: "Some of my random thoughts, mostly about technology",
		Author:      &feeds.Author{Name: "Satyajit Ranjeev", Email: "s@ranjeev.in"},
	}
	host := "https://" + hostname
	if !tls {
		host = "http://" + hostname
	}
	for i := len(pages) - 1; i >= 0; i-- {
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       pages[i].Title,
			Link:        &feeds.Link{Href: fmt.Sprintf("%s%s", host, pages[i].URL)},
			Description: pages[i].Markdown,
			Created:     pages[i].Date,
		})
	}

	rss, err := feed.ToRss()
	if err != nil {
		panic(err)
	}

	content := []byte(rss)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Write(content)
	})
}

func handlePages(pages []*Page) http.Handler {
	parsed := make(map[string][]byte)
	for _, p := range pages {
		parsed[p.URL] = p.Content
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, ok := parsed[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(http.StatusText(http.StatusNotFound)))
			return
		}
		w.Write(a)
	})
}

func getAliases(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening alias file %q %w", filename, err)
	}
	defer f.Close()

	var aliases map[string]string
	if err := json.NewDecoder(f).Decode(&aliases); err != nil {
		return nil, fmt.Errorf("alias json decoding: %w", err)
	}
	return aliases, nil
}

func redirect(host string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		target := "https://" + r.Host + r.URL.Path
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		log.Printf("redirect to: %s", target)
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}
}

// ResponseWriter is a wrapper for http.ResponseWriter to get the written http status code
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (r *ResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.statusCode = statusCode
}

func (r *ResponseWriter) StatusCode() int {
	if r.statusCode == 0 {
		return http.StatusOK
	}
	return r.statusCode
}
