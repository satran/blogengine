package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
)

// FileSystem custom file system handler
type FileSystem struct {
	sync.Mutex
	fs    http.FileSystem
	cache map[string]*File
}

func newFileSystem(dir string) *FileSystem {
	return &FileSystem{
		fs:    http.Dir(dir),
		cache: make(map[string]*File),
	}
}

// Open opens file
func (fs FileSystem) Open(path string) (http.File, error) {
	fs.Lock()
	defer fs.Unlock()
	f, ok := fs.cache[path]
	if ok {
		// Make sure the file can be read from first byte, otherwise images will not be rendered every other try
		_, _ = f.Seek(0, io.SeekStart)
		return f, nil
	}
	f, err := newFile(path, fs.fs)
	if err != nil {
		return nil, err
	}
	fs.cache[path] = f
	return f, nil
}

type File struct {
	*bytes.Reader
	stat  os.FileInfo
	files []os.FileInfo
}

func newFile(path string, fs http.FileSystem) (*File, error) {
	f, err := fs.Open(path)
	if err != nil {
		return nil, err
	}

	var files []os.FileInfo
	s, err := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := fs.Open(index); err != nil {
			return nil, err
		}
		files, err = f.Readdir(-1)
		if err != nil {
			return nil, err
		}
	}

	by, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return &File{
		Reader: bytes.NewReader(by),
		stat:   s,
		files:  files,
	}, nil
}

func (f *File) Close() error {
	return nil
}

func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	return f.files, nil
}

func (f *File) Stat() (os.FileInfo, error) {
	return f.stat, nil
}
