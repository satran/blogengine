package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"time"

	"github.com/russross/blackfriday"
)

type Page struct {
	Path    string
	Title   string
	Date    time.Time
	Content []byte
}

// parsePage returns
func parsePage(page io.Reader) (*Page, error) {
	p := &Page{}
	r := bufio.NewReader(page)
	n := 0
	var markdown template.HTML
	for {
		// the first two lines are use for meta data. The first line is always the title.
		// The second line is the date in the format dd/mm/yyyy.
		// Also the prefix is ignored, I'm assuming I will not create really long lines
		line, _, err := r.ReadLine()
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read page: %w", err)
		}
		n++
		if n == 1 {
			p.Title = string(line)
			continue
		}
		if n == 2 {
			p.Date = time.Now()
		}
		if err == io.EOF {
			break
		}

		content, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("read content: %w", err)
		}
		// the template.HTML allows embedding HTML in the usually escaped HTML during template execution
		markdown = template.HTML(string(blackfriday.Run(content)))
		break
	}
	wr := &bytes.Buffer{}
	data := map[string]interface{}{
		"Title": p.Title,
		"Body":  markdown,
		"Date":  p.Date,
	}
	if err := pageTmpl.Execute(wr, data); err != nil {
		return nil, fmt.Errorf("template parsing: %w", err)
	}
	p.Content = wr.Bytes()
	return p, nil
}

var pageTmpl = template.Must(template.New("foo").Parse(`<!DOCTYPE html>
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
        max-width: 700px;
        margin: 0;
        padding: 1em;
        font-family: 'Helvetica', 'Arial', sans-serif;
        line-height: 1.4;
        color: #333;
        padding-bottom: 100%;
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
<h1 class="title">{{.Title}}</h1>
<p class="date">{{.Date}}</p1>
{{ .Body }}
</div>
</body>
`))
