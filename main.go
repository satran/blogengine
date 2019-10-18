package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	c := config{
		Host:         os.Getenv("HOSTNAME"),
		Cert:         os.Getenv("CERT"),
		Key:          os.Getenv("KEY"),
		PagesDir:     os.Getenv("PAGES"),
		StaticDir:    os.Getenv("STATIC"),
		AliasFile:    os.Getenv("ALIAS"),
		MetricsToken: os.Getenv("METRICS_TOKEN"),
		UseTLS:       true,
	}
	if c.MetricsToken == "" {
		//log.Fatal("can't run without metrics token")
	}
	if c.Host == "" {
		c.Host = "localhost:8080"
	}
	if c.Cert == "" && c.Key == "" {
		c.UseTLS = false
	}
	if err := run(c); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	Host         string
	Cert         string
	Key          string
	PagesDir     string
	StaticDir    string
	AliasFile    string
	MetricsToken string
	UseTLS       bool
}

func run(c config) error {
	articles, err := parse(c.PagesDir)
	if err != nil {
		return fmt.Errorf("parse articles: %w", err)
	}
	aliases, err := getAliases(c.AliasFile)
	if err != nil {
		return fmt.Errorf("load aliases: %w", err)
	}

	requestDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Time spent in the service",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 25, 50, 100, 250},
	})
	totalRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests",
			Help: "HTTP Requests",
		},
		[]string{"code", "method", "path"},
	)
	prometheus.DefaultRegisterer.MustRegister(requestDuration, totalRequests)

	m := http.ServeMux{}
	m.HandleFunc("/", instrument(requestDuration, totalRequests, handleIndex(articles["/"], aliases)))
	m.HandleFunc("/b/", instrument(requestDuration, totalRequests, handleArticle(articles)))
	m.Handle("/s/", http.StripPrefix("/s/", http.FileServer(newFileSystem(c.StaticDir))))
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

func instrument(duration prometheus.Histogram, total *prometheus.CounterVec, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := r.URL.Path
		wr := &ResponseWriter{ResponseWriter: w}
		handler(wr, r)
		since := time.Since(start)
		statusCode := strconv.Itoa(wr.StatusCode())
		duration.Observe(since.Seconds())
		total.WithLabelValues(statusCode, strings.ToUpper(r.Method), path).Inc()
		log.Println(since, r.Method, path)
	}
}

func handleIndex(index []byte, aliases map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
}

func handleArticle(articles map[string][]byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := articles[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(http.StatusText(http.StatusNotFound)))
			return
		}
		w.Write(a)
	}
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
