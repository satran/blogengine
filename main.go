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
		FeedFile:  os.Getenv("FEED"),
		UseTLS:    true,
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
	FeedFile  string
	UseTLS    bool
}

func run(c config) error {
	articles, err := parse(c.PagesDir)
	if err != nil {
		return fmt.Errorf("parse articles: %w", err)
	}

	m := mux{
		feedFile: c.FeedFile,
		d:        articles,
		images:   http.StripPrefix("/images/", http.FileServer(FileSystem{http.Dir(c.ImagesDir)})),
	}
	srv := &http.Server{
		ReadTimeout:  time.Second,
		WriteTimeout: 2 * time.Second,
		Handler:      &m,
	}
	srv.SetKeepAlivesEnabled(false)
	if c.UseTLS {
		srv.Addr = ":443"
		go http.ListenAndServe(":80", http.HandlerFunc(redirect))
		err = srv.ListenAndServeTLS(c.Cert, c.Key)
	} else {
		srv.Addr = ":80"
		err = srv.ListenAndServe()
	}
	return err
}

func index(w http.ResponseWriter, req *http.Request) {
	// all calls to unknown url paths should return 404
	if req.URL.Path != "/" {
		log.Printf("404: %s", req.URL.String())
		http.NotFound(w, req)
		return
	}
	http.ServeFile(w, req, "index.html")
}

func redirect(w http.ResponseWriter, req *http.Request) {
	target := "https://" + req.Host + req.URL.Path
	if len(req.URL.RawQuery) > 0 {
		target += "?" + req.URL.RawQuery
	}
	log.Printf("redirect to: %s", target)
	http.Redirect(w, req, target, http.StatusTemporaryRedirect)
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
	feedFile string
	images   http.Handler
	d        map[string][]byte
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL.Path)
	if r.URL.Path == "/feed.xml" {
		http.ServeFile(w, r, m.feedFile)
		return
	}
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
