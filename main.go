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
)

func main() {
	dir := os.Getenv("DIR")
	if dir == "" {
		dir = "posts"
	}
	articles, err := parse(dir)
	if err != nil {
		log.Fatal(err)
	}
	for n := range articles {
		log.Println(n)
	}

	m := mux{d: articles}
	if err := http.ListenAndServe(":8080", &m); err != nil {
		log.Fatal(err)
	}
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
		articleList = append(articleList, fmt.Sprintf(`<p><a href="/%s">%s</a></p>`, name, name))
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
