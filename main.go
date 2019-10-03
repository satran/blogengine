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

	"github.com/russross/blackfriday"
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
	for _, f := range files {
		name := f.Name()
		raw, err := ioutil.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}
		// the template.HTML allows embedding HTML in the usually escaped HTML during template execution
		parsed := template.HTML(string(blackfriday.Run(raw)))
		wr := &bytes.Buffer{}
		if err := postTmpl.Execute(wr, map[string]interface{}{"Title": name, "Body": parsed}); err != nil {
			return nil, fmt.Errorf("template parsing: %w", err)
		}
		articles[name] = wr.Bytes()
	}
	return articles, nil
}

type mux struct {
	d map[string][]byte
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL.Path[1:])
	a, ok := m.d[r.URL.Path[1:]]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte("not found")); err != nil {
			log.Println(err)
		}
		return
	}
	w.Write(a)
}

var postTmpl = template.Must(template.New("foo").Parse(`<!DOCTYPE html>
<html>
<head>
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <meta http-equiv="cache-control" content="no-cache">
    <title>{{ .Title }}</title>
    <link rel="shortcut icon" href="/images/favicon.png" />
    <style>
    body {
        background: #fafafa;
        margin: 0;
        padding: 0;
    }
    .container {
        background: #fff;
        max-width: 600px;
        font-family: 'Helvetica', 'Arial', sans-serif;
        line-height: 1.3;
        color: #333;
        margin: 0;
        padding: 1em;
    }
    a {
        color: #333;
    }
    a:visited {
        color: #777;
    }
    @media (prefers-color-scheme: dark) {
        body {
            background: #000;
        }
        .container {
            background: #181818;
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
{{ .Body }}
</div>
</body>
`))
