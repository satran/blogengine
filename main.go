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
		Host:      os.Getenv("HOSTNAME"),
		Cert:      os.Getenv("CERT"),
		Key:       os.Getenv("KEY"),
		PagesDir:  os.Getenv("PAGES"),
		StaticDir: os.Getenv("STATIC"),
		UseTLS:    true,
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
	Host      string
	Cert      string
	Key       string
	PagesDir  string
	StaticDir string
	UseTLS    bool
}

func run(c config) error {
	articles, err := parse(c.PagesDir)
	if err != nil {
		return fmt.Errorf("parse articles: %w", err)
	}

	m := mux{
		host:   c.Host,
		d:      articles,
		static: http.FileServer(FileSystem{http.Dir(c.StaticDir)}),
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
		srv.Addr = ":80"
		err = srv.ListenAndServe()
	}
	return err
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

var indexTmpl = template.Must(template.New("foo").Parse(`<!DOCTYPE html>
<html>
<head>
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <meta http-equiv="cache-control" content="no-cache">
    <title>Satyajit Ranjeev</title>
    <link rel="shortcut icon" href="/images/favicon.png" />
    <style>
    body {
        background: #fff;
        margin: 0;
        padding: 0;
    }
    .container {
        max-width: 600px;
	min-height: 100%;
        margin: 0 auto;
        padding: 1em;
        font-family: 'Helvetica Neue', 'Arial', sans-serif;
        line-height: 1.5;
        color: #333;
	text-rendering: optimizeLegibility;
    }
    .container img {
    	max-width: 100%;
    }
    .title {
        padding: 0;
	margin: 0;
	text-align: center;
	font-weight: normal;
    }
    span.date {
	font-style: italic;
    }
    a {
        color: #333;
    }
    a:visited {
        color: #333;
    }
    @media (prefers-color-scheme: dark) {
        body {
            background: #181818;
        }
        .container {
            color: #ddd;
        }
        a {
            color: #ddd;
        }
        a:visited {
            color: #aaa;
        }

    }
    @media print{
		body{
			max-width:none
		}
    }
    </style>
</head>
<body>
<div class="container">
<p> Hi! I am Satyajit Ranjeev. I am a computer programmer who loves to solve problems. I currently work at <a href="https://solarisbank.de">solarisBank</a> as an engineer.</p>
<p class="date"></p1>
<h3>Articles</h3>
{{ range . }}
<p>
    <a href="{{.Path}}">{{.Title}}</a>
    <span class="date"> - {{ .Date.Format "2, Jan 2006" }}</span>
</p>
{{ end }}
</div>
</body>
`))

func renderIndex(articles []*Page) ([]byte, error) {
	wr := &bytes.Buffer{}
	if err := indexTmpl.Execute(wr, articles); err != nil {
		return nil, fmt.Errorf("template parsing: %w", err)
	}
	return wr.Bytes(), nil
}

type mux struct {
	static http.Handler
	d      map[string][]byte
	host   string
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL.Path)
	if r.Host != m.host {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	a, ok := m.d[r.URL.Path]
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
