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
	cert := os.Getenv("CERT")
	key := os.Getenv("KEY")
	dir := os.Getenv("DIR")
	if dir == "" {
		dir = "posts"
	}
	c := config{
		Cert: cert,
		Key:  key,
		Dir:  dir,
	}
	if cert == "" && key == "" {
		c.UseTLS = false
	}
	if err := run(c); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	Cert   string
	Key    string
	Dir    string
	UseTLS bool
}

func run(c config) error {
	articles, err := parse(c.Dir)
	if err != nil {
		return fmt.Errorf("parse articles: %w", err)
	}

	m := mux{d: articles}
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
		p, err := parsePage(f)
		if err != nil {
			return nil, fmt.Errorf("parse page: %w", err)
		}
		articles["/"+name] = p.Content
		articleList = append(articleList, fmt.Sprintf(`<p>%s <a href="/%s">%s</a></p>`, p.Date.Format("Jan 2 2006"), name, p.Title))
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
	d map[string][]byte
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL.Path)
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
