package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/matt-deboer/go-marathon"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type MarathonUris []string

func (i *MarathonUris) Set(value string) error {
    *i = append(*i, value)
    return nil
}

func (i *MarathonUris) String() string {
	return "Marathon URIs to scrape"
}

var marathonUris MarathonUris

var (
	listenAddress = flag.String(
		"web.listen-address", ":9088",
		"Address to listen on for web interface and telemetry.")

	metricsPath = flag.String(
		"web.telemetry-path", "/metrics",
		"Path under which to expose metrics.")
)

func marathonConnect(uri *url.URL) error {
	config := marathon.NewDefaultConfig()
	config.URL = uri.String()

	if uri.User != nil {
		if passwd, ok := uri.User.Password(); ok {
			config.HTTPBasicPassword = passwd
			config.HTTPBasicAuthUser = uri.User.Username()
		}
	}
	config.HTTPClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	log.Debugln("Connecting to Marathon")
	client, err := marathon.NewClient(config)
	if err != nil {
		return err
	}

	info, err := client.Info()
	if err != nil {
		return err
	}

	log.Debugf("Connected to Marathon! Name=%s, Version=%s\n", info.Name, info.Version)
	return nil
}

func CheckMarathonConnection(uri *url.URL) {
	retryTimeout := time.Duration(10 * time.Second)
	for {
		err := marathonConnect(uri)
		if err == nil {
			break
		}

		log.Debugf("Problem connecting to Marathon: %v", err)
		log.Infof("Couldn't connect to Marathon! Trying again in %v", retryTimeout)
		time.Sleep(retryTimeout)
	}
}

func InstanceId(url *url.URL) string {
	if url.Port() == "" {
		return url.Hostname()
	}
	return fmt.Sprintf("%s:%s", url.Hostname(), url.Port())
}

func main() {
	flag.Var(&marathonUris, "marathon.uri", "URI of Marathon")
	flag.Parse()

	exporter := &Exporter{}

	for _, marathonUri := range marathonUris {
		uri, err := url.Parse(marathonUri)
		if err != nil {
			log.Fatal(err)
		}
		CheckMarathonConnection(uri)
		singleExporter := NewSingleExporter(&scraper{uri}, InstanceId(uri), defaultNamespace)
		exporter.exporters = append(exporter.exporters, singleExporter)
	}

	prometheus.MustRegister(exporter)

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

	log.Info("Starting Server: ", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
