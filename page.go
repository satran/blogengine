package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
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

var findDate = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}`).FindAllString

// parsePage returns
func parsePage(name string, page io.Reader) (*Page, error) {
	name = filepath.Base(name)
	// Jekyll served .html
	name = strings.Replace(name, ".md", ".html", -1)
	// Date is assumed to be the first part of the name in format dd/mm/yyyy.
	dates := findDate(name, 1)
	if len(dates) == 0 {
		return nil, errors.New("couldn't find date in file name; file name should be of the format yyyy-mm-dd-name_of_file")
	}
	// We convert the date into yyyy/mm/dd as this is how jekyll formatted
	// the page path. Also note that we can't have filenames with / and that's
	// why the - is used.
	dateStr := strings.Replace(dates[0], "-", "/", -1)
	nameWithoutDate := strings.TrimPrefix(name, dates[0]+"-")
	date, err := time.Parse("2006/01/02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse date: %w", err)
	}
	p := &Page{
		Path: "/" + dateStr + "/" + nameWithoutDate,
		Date: date,
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
