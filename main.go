package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
		log.Fatal("can't run without metrics token")
	}
	if c.Host == "" {
		c.Host = "localhost"
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

	m := mux{
		host:        c.Host,
		d:           articles,
		alias:       aliases,
		static:      http.FileServer(FileSystem{http.Dir(c.StaticDir)}),
		metrics:     promhttp.Handler(),
		bearerToken: c.MetricsToken,
	}
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

func parse(dir string) (map[string][]byte, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	articles := make(map[string][]byte)
	var index []*Page
	for _, f := range files {
		name := f.Name()
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}
		defer f.Close()
		p, err := parsePage(f.Name(), f)
		if err != nil {
			return nil, fmt.Errorf("parse page %s: %w", f.Name(), err)
		}
		articles[p.Path] = p.Content
		index = append(index, p)
	}
	articles["/"], err = renderIndex(index)
	if err != nil {
		return nil, fmt.Errorf("render index: %w", err)
	}
	return articles, nil
}

var indexTmpl = template.Must(template.ParseFiles("templates/index.html"))

func renderIndex(articles []*Page) ([]byte, error) {
	// sort the articles by the lastest at the top
	sort.Sort(sort.Reverse(Pages(articles)))
	wr := &bytes.Buffer{}
	if err := indexTmpl.Execute(wr, articles); err != nil {
		return nil, fmt.Errorf("template parsing: %w", err)
	}
	return wr.Bytes(), nil
}

type mux struct {
	static      http.Handler
	metrics     http.Handler
	bearerToken string
	alias       map[string]string
	d           map[string][]byte
	host        string
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Println(r.Method, path)
	if path == "/metrics" {
		reqToken := r.Header.Get("Authorization")
		splitToken := strings.Split(reqToken, " ")
		if len(splitToken) == 2 && splitToken[1] == m.bearerToken {
			m.metrics.ServeHTTP(w, r)
			return
		}
		log.Printf("unknown token: %q, %q", splitToken[1], m.bearerToken)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Host != m.host {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	alias, ok := m.alias[path]
	if ok {
		if len(r.URL.RawQuery) > 0 {
			alias += "?" + r.URL.RawQuery
		}
		log.Printf("redirect to: %s", alias)
		http.Redirect(w, r, alias, http.StatusTemporaryRedirect)
		return
	}
	a, ok := m.d[path]
	if ok {
		w.Write(a)
		return
	}
	m.static.ServeHTTP(w, r)
	return
}

// FileSystem custom file system handler
type FileSystem struct {
	fs http.FileSystem
}

// Open opens file
func (fs FileSystem) Open(path string) (http.File, error) {
	f, err := fs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := fs.fs.Open(index); err != nil {
			return nil, err
		}
	}

	return f, nil
}
