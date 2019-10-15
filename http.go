package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Mux struct {
	static      http.Handler
	metrics     http.Handler
	bearerToken string
	alias       map[string]string
	d           map[string][]byte
	host        string
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	path := r.URL.Path
	wr := &ResponseWriter{ResponseWriter: w}
	defer func() {
		since := time.Since(start)
		statusCode := strconv.Itoa(wr.StatusCode())
		requestDuration.Observe(since.Seconds())
		totalRequests.WithLabelValues(statusCode, strings.ToUpper(r.Method), path).Inc()
		log.Println(since, r.Method, path)
	}()
	if path == "/metrics" {
		chunks := strings.Split(r.Header.Get("Authorization"), " ")
		if len(chunks) == 2 && chunks[1] == m.bearerToken {
			m.metrics.ServeHTTP(wr, r)
			return
		}
		log.Printf("unknown token: %q", chunks[1])
		wr.WriteHeader(http.StatusUnauthorized)
		return
	}

	if r.Host != m.host {
		log.Println("unknown host: ", r.Host)
		wr.WriteHeader(http.StatusNotFound)
		return
	}

	a, ok := m.d[path]
	if ok {
		wr.Write(a)
		return
	}

	alias, ok := m.alias[path]
	if ok {
		if len(r.URL.RawQuery) > 0 {
			alias += "?" + r.URL.RawQuery
		}
		log.Printf("redirect to: %s", alias)
		http.Redirect(wr, r, alias, http.StatusTemporaryRedirect)
		return
	}

	m.static.ServeHTTP(wr, r)
	return
}

// ResponseWriter is a wrapper for http.ResponseWriter to get the written http status code
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (r *ResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.statusCode = statusCode
}

func (r *ResponseWriter) StatusCode() int {
	if r.statusCode == 0 {
		return http.StatusOK
	}
	return r.statusCode
}
