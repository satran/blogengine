package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/russross/blackfriday"
)

func parse(templatesDir, dir string) ([]*Page, error) {
	tmpl := template.Must(template.ParseFiles(filepath.Join(templatesDir, "page.html")))
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var pages []*Page
	for _, f := range files {
		name := f.Name()
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}
		defer f.Close()
		p, err := parsePage(tmpl, f.Name(), f)
		if err != nil {
			return nil, fmt.Errorf("parse page %s: %w", f.Name(), err)
		}
		pages = append(pages, p)
	}
	return pages, nil
}

func parsePage(tmpl *template.Template, name string, page io.Reader) (*Page, error) {
	name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	p := &Page{
		URL: "/b/" + name,
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
		markdown = template.HTML(string(blackfriday.Run(content, blackfriday.WithExtensions(blackfriday.Footnotes | blackfriday.CommonExtensions))))
		break
	}
	wr := &bytes.Buffer{}
	data := map[string]interface{}{
		"Title": p.Title,
		"Body":  markdown,
		"Date":  p.Date,
	}
	if err := tmpl.Execute(wr, data); err != nil {
		return nil, fmt.Errorf("template parsing: %w", err)
	}
	p.Content = wr.Bytes()
	p.Markdown = string(markdown)
	return p, nil
}

type Page struct {
	URL      string
	Title    string
	Date     time.Time
	Content  []byte
	Markdown string
}

type Pages []*Page

func (p Pages) Len() int           { return len(p) }
func (p Pages) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Pages) Less(i, j int) bool { return p[i].Date.Before(p[j].Date) }
