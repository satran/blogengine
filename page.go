package main

import (
	"html/template"
	"io"
)

type Page struct {
	Path    string
	Title   string
	Date    string
	Content template.HTML
}

// parsePage returns
func parsePage(r io.Reader) (*Page, error) {

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
{{ .Body }}
</div>
</body>
`))
