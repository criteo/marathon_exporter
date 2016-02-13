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
	Gauges         *GaugeContainer
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
			e.scrapeGauges(element, ch)
		case "histograms":
			e.scrapeHistograms(element, ch)
		case "meters":
			e.scrapeMeters(element, ch)
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

		data := element.Path("count").Data()
		count, ok := data.(float64)
		if !ok {
			log.Debugf("Bad conversion! Skipping counter %s with count %v\n", name, data)
			continue
		}

		log.Debugf("Adding counter %s with count %v\n", name, count)
		counter := e.Counters.GetOrCreate(name)
		counter.WithLabelValues().Set(count)
		counter.Collect(ch)
	}
}

func (e *Exporter) scrapeGauges(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		log.Debugf("Found gauge metric %s\n", key)
		name := metricName(key)

		data := element.Path("value").Data()
		value, ok := data.(float64)
		if !ok {
			log.Debugf("Bad conversion! Skipping gauge %s with value %v\n", name, data)
			continue
		}

		log.Debugf("Adding gauge %s with value %v\n", name, value)
		gauge := e.Gauges.GetOrCreate(name)
		gauge.WithLabelValues().Set(value)
		gauge.Collect(ch)
	}
}

func (e *Exporter) scrapeMeters(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		log.Debugf("Found meter metric %s\n", key)
		name := metricName(key)
		e.scrapeMeter(name, element, ch)
	}
}

func (e *Exporter) scrapeMeter(name string, json *gabs.Container, ch chan<- prometheus.Metric) {
	count, ok := json.Path("count").Data().(float64)
	if !ok {
		log.Debugf("Bad meter! %s has no count\n", name)
		return
	}

	log.Debugf("Adding meter %s with count %v\n", name, count)
	counter := e.Counters.GetOrCreate(name + "_count")
	counter.WithLabelValues().Set(count)
	counter.Collect(ch)

	gauge := e.Gauges.GetOrCreate(name, "rate")
	properties, _ := json.ChildrenMap()
	for propName, property := range properties {
		if !strings.Contains(propName, "rate") {
			continue
		}

		if value, ok := property.Data().(float64); ok {
			gauge.WithLabelValues(
				rateName(propName),
			).Set(value)
		}
	}

	gauge.Collect(ch)
}

func (e *Exporter) scrapeHistograms(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		log.Debugf("Found histogram metric %s\n", key)
		name := metricName(key)
		e.scrapeHistogram(name, element, ch)
	}
}

func (e *Exporter) scrapeHistogram(name string, json *gabs.Container, ch chan<- prometheus.Metric) {
	count, ok := json.Path("count").Data().(float64)
	if !ok {
		log.Debugf("Bad histogram! %s has no count\n", name)
		return
	}

	log.Debugf("Adding histogram %s with count %v\n", name, count)
	counter := e.Counters.GetOrCreate(name + "_count")
	counter.WithLabelValues().Set(count)
	counter.Collect(ch)

	percentiles := e.Gauges.GetOrCreate(name, "percentile")
	properties, _ := json.ChildrenMap()
	for propName, property := range properties {
		switch propName {
		case "p50", "p75", "p95", "p98", "p99", "p999":
			if value, ok := property.Data().(float64); ok {
				percentiles.WithLabelValues(
					"0." + propName[1:],
				).Set(value)
			}
		case "max", "min", "mean", "stdev":
			if value, ok := property.Data().(float64); ok {
				gauge := e.Gauges.GetOrCreate(name + "_" + propName)
				gauge.WithLabelValues().Set(value)
				gauge.Collect(ch)
			}
		}

	}

	percentiles.Collect(ch)
}

func rateName(originalRate string) (name string) {
	switch originalRate {
	case "m1_rate":
		name = "1m"
	case "m5_rate":
		name = "5m"
	case "m15_rate":
		name = "15m"
	default:
		name = strings.TrimSuffix(originalRate, "_rate")
	}
	return
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
		Gauges:   NewGaugeContainer(),
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
