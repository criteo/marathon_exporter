package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jeffail/gabs"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const namespace = "marathon"

type Exporter struct {
	uri            *url.URL
	metricsVersion string
	duration       prometheus.Gauge
	scrapeError    prometheus.Gauge
	totalErrors    prometheus.Counter
	totalScrapes   prometheus.Counter
	Counters       *CounterContainer
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	log.Debugln("Describing metrics")
	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	log.Debugln("Collecting metrics")
	e.scrape(ch)

	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.totalErrors
	ch <- e.scrapeError
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()

	var err error
	defer func(begin time.Time) {
		e.duration.Set(time.Since(begin).Seconds())
		if err == nil {
			e.scrapeError.Set(0)
		} else {
			e.totalErrors.Inc()
			e.scrapeError.Set(1)
		}
	}(time.Now())

	client := &http.Client{
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

	response, err := client.Get(fmt.Sprintf("%v/metrics", e.uri))
	if err != nil {
		log.Debugf("Problem connecting to metrics endpoint: %v\n", err)
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		log.Debugf("Problem reading metrics response body: %v\n", err)
		return
	}

	json, err := gabs.ParseJSON(body)
	if err != nil {
		log.Debugf("Problem parsing metrics response body: %v\n", err)
		return
	}

	elements, err := json.ChildrenMap()
	for key, element := range elements {
		switch key {
		case "message":
			log.Debugf("Problem collecting metrics: %s\n", element.Data().(string))
			return
		case "version":
			if e.metricsVersion == "" {
				e.metricsVersion = element.Data().(string)
				log.Infof("Collecting Marathon metrics version: %s\n", e.metricsVersion)
			}
		case "counters":
			e.scrapeCounters(element, ch)
		case "gauges":
			log.Debugln("Found gauges")
		case "histograms":
			log.Debugln("Found historgrams")
		case "meters":
			log.Debugln("Found meters")
		case "timers":
			log.Debugln("Found timers")
		}
	}

}

func (e *Exporter) scrapeCounters(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		log.Debugf("Found counter metric %s\n", key)
		name := metricName(key)
		value := element.Path("count").Data().(float64)

		log.Debugf("Adding value %v to counter %s\n", value, name)
		counter := e.Counters.GetOrCreate(name, prometheus.Labels{})
		counter.Add(value)
		ch <- counter
	}
}

func metricName(originalName string) (name string) {
	name = strings.ToLower(originalName)
	name = strings.Replace(name, ".", "_", -1)
	name = strings.Replace(name, "-", "_", -1)
	name = strings.Replace(name, "$", "_", -1)
	return
}

func NewExporter(uri *url.URL) *Exporter {
	return &Exporter{
		uri:      uri,
		Counters: NewCounterContainer(),
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from Marathon.",
		}),
		scrapeError: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from Marathon resulted in an error (1 for error, 0 for success).",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrapes_total",
			Help:      "Total number of times Marathon was scraped for metrics.",
		}),
		totalErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "errors_total",
			Help:      "Total number of times the exporter experienced errors collecting Marathon metrics.",
		}),
	}
}
