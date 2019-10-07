package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/russross/blackfriday"
)

type Page struct {
	Path    string
	Title   string
	Date    time.Time
	Content []byte
}

func parsePage(name string, page io.Reader) (*Page, error) {
	name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	p := &Page{
		Path: "/b/" + name,
	}
	r := bufio.NewReader(page)
	n := 0
	var markdown template.HTML
	for {
		// the first two lines are use for meta data. The first line is always the title.
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
			p.Date, err = time.Parse("02/01/2006", string(line))
			if err != nil {
				return nil, fmt.Errorf("couldn't parse date: %w", err)
			}
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

var pageTmpl = template.Must(template.ParseFiles("templates/page.html"))
