package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	listenAddress = flag.String(
		"web.listen-address", ":9099",
		"Address to listen on for web interface and telemetry.")

	metricsPath = flag.String(
		"web.telemetry-path", "/metrics",
		"Path under which to expose metrics.")

	marathonUri = flag.String(
		"marathon.uri", "http://marathon.mesos:8080",
		"URI of Marathon")
)

func main() {
	flag.Parse()
	uri, err := url.Parse(*marathonUri)
	if err != nil {
		log.Fatal(err)
	}

	exporter := NewExporter(uri)
	prometheus.MustRegister(exporter)

	log.Println("Starting Server:", *listenAddress)
	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Marathon Exporter</title></head>
             <body>
             <h1>Marathon Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
