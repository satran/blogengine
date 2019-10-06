package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	c := config{
		Cert:      os.Getenv("CERT"),
		Key:       os.Getenv("KEY"),
		PagesDir:  os.Getenv("PAGES"),
		ImagesDir: os.Getenv("IMAGES"),
	}
	if c.Cert == "" && c.Key == "" {
		c.UseTLS = false
	}
	if err := run(c); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	Cert      string
	Key       string
	PagesDir  string
	ImagesDir string
	UseTLS    bool
}

func run(c config) error {
	articles, err := parse(c.PagesDir)
	if err != nil {
		return fmt.Errorf("parse articles: %w", err)
	}

	m := mux{
		d:      articles,
		images: http.StripPrefix("/images/", http.FileServer(FileSystem{http.Dir(c.ImagesDir)})),
	}
	srv := &http.Server{
		ReadTimeout:  time.Second,
		WriteTimeout: 2 * time.Second,
		Handler:      &m,
	}
	srv.SetKeepAlivesEnabled(false)
	if c.UseTLS {
		srv.Addr = ":443"
		err = srv.ListenAndServeTLS(c.Cert, c.Key)
	} else {
		srv.Addr = ":80"
		err = srv.ListenAndServe()
	}
	return err
}

func parse(dir string) (map[string][]byte, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	articles := make(map[string][]byte)
	var articleList []string
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
		println(p.Path)
		articles[p.Path] = p.Content
		articleList = append(articleList, fmt.Sprintf(`<p>%s <a href="%s">%s</a></p>`, p.Date.Format("Jan 2 2006"), p.Path, p.Title))
	}
	parsed := template.HTML(strings.Join(articleList, "\n"))
	wr := &bytes.Buffer{}
	if err := pageTmpl.Execute(wr, map[string]interface{}{"Title": "Articles", "Body": parsed}); err != nil {
		return nil, fmt.Errorf("template parsing: %w", err)
	}
	articles["/articles"] = wr.Bytes()
	return articles, nil
}

type mux struct {
	images http.Handler
	d      map[string][]byte
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL.Path)
	if len(r.URL.Path) > len("/images") &&
		r.URL.Path[:len("/images")] == "/images" {
		m.images.ServeHTTP(w, r)
		return
	}
	a, ok := m.d[r.URL.Path]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte("not found")); err != nil {
			log.Println(err)
		}
		return
	}
	w.Write(a)
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
