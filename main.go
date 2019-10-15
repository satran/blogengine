package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Time spent in the service",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 25, 50, 100, 250},
	})

	totalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests",
			Help: "HTTP Requests",
		},
		[]string{"code", "method", "path"},
	)
)

func main() {
	c := config{
		Host:         os.Getenv("HOSTNAME"),
		Cert:         os.Getenv("CERT"),
		Key:          os.Getenv("KEY"),
		PagesDir:     os.Getenv("PAGES"),
		StaticDir:    os.Getenv("STATIC"),
		AliasFile:    os.Getenv("ALIAS"),
		MetricsToken: os.Getenv("METRICS_TOKEN"),
		UseTLS:       true,
	}
	if c.MetricsToken == "" {
		//log.Fatal("can't run without metrics token")
	}
	if c.Host == "" {
		c.Host = "localhost:8080"
	}
	if c.Cert == "" && c.Key == "" {
		c.UseTLS = false
	}
	if err := run(c); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	Host         string
	Cert         string
	Key          string
	PagesDir     string
	StaticDir    string
	AliasFile    string
	MetricsToken string
	UseTLS       bool
}

func run(c config) error {
	articles, err := parse(c.PagesDir)
	if err != nil {
		return fmt.Errorf("parse articles: %w", err)
	}
	aliases, err := getAliases(c.AliasFile)
	if err != nil {
		return fmt.Errorf("load aliases: %w", err)
	}
	prometheus.DefaultRegisterer.MustRegister(requestDuration, totalRequests)

	m := Mux{
		host:        c.Host,
		d:           articles,
		alias:       aliases,
		static:      http.FileServer(FileSystem{http.Dir(c.StaticDir)}),
		metrics:     promhttp.Handler(),
		bearerToken: c.MetricsToken,
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
		srv.Addr = ":8080"
		err = srv.ListenAndServe()
	}
	return err
}

func getAliases(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening alias file %q %w", filename, err)
	}
	defer f.Close()

	var aliases map[string]string
	if err := json.NewDecoder(f).Decode(&aliases); err != nil {
		return nil, fmt.Errorf("alias json decoding: %w", err)
	}
	return aliases, nil
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
