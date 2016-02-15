package main

import (
	"crypto/tls"
	"errors"
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
	uri          *url.URL
	duration     prometheus.Gauge
	scrapeError  prometheus.Gauge
	totalErrors  prometheus.Counter
	totalScrapes prometheus.Counter
	Counters     *CounterContainer
	Gauges       *GaugeContainer
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

	e.scrapeMetrics(json, ch)
}

func (e *Exporter) scrapeMetrics(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		switch key {
		case "message":
			log.Errorf("Problem collecting metrics: %s\n", element.Data().(string))
			return
		case "version":
			data := element.Data()
			version, ok := data.(string)
			if !ok {
				log.Errorf(fmt.Sprintf("Bad conversion! Unexpected value \"%v\" for version\n", data))
			} else {
				gauge, _ := e.Gauges.Fetch("metrics_version", "Marathon metrics version", "version")
				gauge.WithLabelValues(version).Set(1)
				gauge.Collect(ch)
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
			e.scrapeTimers(element, ch)
		}
	}
}

func (e *Exporter) scrapeCounters(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		counter, err := e.scrapeCounter(key, element)
		if err == nil {
			counter.Collect(ch)
		} else {
			log.Debug(err)
		}
	}
}

func (e *Exporter) scrapeCounter(key string, json *gabs.Container) (prometheus.Collector, error) {
	data := json.Path("count").Data()
	count, ok := data.(float64)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Bad conversion! Unexpected value \"%v\" for counter %s\n", data, key))
	}

	name := renameMetric(key)
	help := fmt.Sprintf(counterHelp, key)
	counter, new := e.Counters.Fetch(name, help)
	counter.WithLabelValues().Set(count)
	if new {
		log.Infof("Added counter %s with initial count %v\n", name, count)
	}
	return counter, nil
}

func (e *Exporter) scrapeGauges(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		gauge, err := e.scrapeGauge(key, element)
		if err == nil {
			gauge.Collect(ch)
		} else {
			log.Debug(err)
		}
	}
}

func (e *Exporter) scrapeGauge(key string, json *gabs.Container) (prometheus.Collector, error) {
	data := json.Path("value").Data()
	value, ok := data.(float64)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Bad conversion! Unexpected value \"%v\" for gauge %s\n", data, key))
	}

	name := renameMetric(key)
	help := fmt.Sprintf(gaugeHelp, key)
	gauge, new := e.Gauges.Fetch(name, help)
	gauge.WithLabelValues().Set(value)
	if new {
		log.Infof("Added gauge %s with initial value %v\n", name, value)
	}
	return gauge, nil
}

func (e *Exporter) scrapeMeters(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		metrics, err := e.scrapeMeter(key, element)
		if err != nil {
			log.Debug(err)
		} else {
			for _, metric := range metrics {
				metric.Collect(ch)
			}
		}
	}
}

func (e *Exporter) scrapeMeter(key string, json *gabs.Container) ([]prometheus.Collector, error) {
	count, ok := json.Path("count").Data().(float64)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Bad meter! %s has no count\n", key))
	}
	units, ok := json.Path("units").Data().(string)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Bad meter! %s has no units\n", key))
	}

	name := renameMetric(key)
	help := fmt.Sprintf(meterHelp, key, units)
	counter, new := e.Counters.Fetch(name+"_count", help)
	counter.WithLabelValues().Set(count)

	gauge, _ := e.Gauges.Fetch(name, help, "rate")
	properties, _ := json.ChildrenMap()
	for key, property := range properties {
		if strings.Contains(key, "rate") {
			if value, ok := property.Data().(float64); ok {
				gauge.WithLabelValues(renameRate(key)).Set(value)
			}
		}
	}

	if new {
		log.Infof("Adding meter %s with initial count %v\n", name, count)
	}
	return []prometheus.Collector{counter, gauge}, nil
}

func (e *Exporter) scrapeHistograms(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		metrics, err := e.scrapeHistogram(key, element)
		if err != nil {
			log.Debug(err)
		} else {
			for _, metric := range metrics {
				metric.Collect(ch)
			}
		}
	}
}

func (e *Exporter) scrapeHistogram(key string, json *gabs.Container) ([]prometheus.Collector, error) {
	count, ok := json.Path("count").Data().(float64)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Bad historgram! %s has no count\n", key))
	}

	name := renameMetric(key)
	help := fmt.Sprintf(histogramHelp, key)
	counter, new := e.Counters.Fetch(name+"_count", help)
	counter.WithLabelValues().Set(count)

	percentiles, _ := e.Gauges.Fetch(name, help, "percentile")
	max, _ := e.Gauges.Fetch(name+"_max", help)
	mean, _ := e.Gauges.Fetch(name+"_mean", help)
	min, _ := e.Gauges.Fetch(name+"_min", help)
	stddev, _ := e.Gauges.Fetch(name+"_stddev", help)

	properties, _ := json.ChildrenMap()
	for key, property := range properties {
		switch key {
		case "p50", "p75", "p95", "p98", "p99", "p999":
			if value, ok := property.Data().(float64); ok {
				percentiles.WithLabelValues("0." + key[1:]).Set(value)
			}
		case "min":
			if value, ok := property.Data().(float64); ok {
				min.WithLabelValues().Set(value)
			}
		case "max":
			if value, ok := property.Data().(float64); ok {
				max.WithLabelValues().Set(value)
			}
		case "mean":
			if value, ok := property.Data().(float64); ok {
				mean.WithLabelValues().Set(value)
			}
		case "stddev":
			if value, ok := property.Data().(float64); ok {
				stddev.WithLabelValues().Set(value)
			}
		}
	}

	if new {
		log.Infof("Adding histogram %s with initial count %v\n", name, count)
	}

	return []prometheus.Collector{counter, percentiles, max, mean, min, stddev}, nil
}

func (e *Exporter) scrapeTimers(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		metrics, err := e.scrapeTimer(key, element)
		if err != nil {
			log.Debug(err)
		} else {
			for _, metric := range metrics {
				metric.Collect(ch)
			}
		}
	}
}

func (e *Exporter) scrapeTimer(key string, json *gabs.Container) ([]prometheus.Collector, error) {
	count, ok := json.Path("count").Data().(float64)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Bad timer! %s has no count\n", key))
	}
	units, ok := json.Path("rate_units").Data().(string)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Bad timer! %s has no units\n", key))
	}

	name := renameMetric(key)
	help := fmt.Sprintf(timerHelp, key, units)
	counter, new := e.Counters.Fetch(name+"_count", help)
	counter.WithLabelValues().Set(count)

	rates, _ := e.Gauges.Fetch(name+"_rate", help, "rate")
	percentiles, _ := e.Gauges.Fetch(name, help, "percentile")
	min, _ := e.Gauges.Fetch(name+"_min", help)
	max, _ := e.Gauges.Fetch(name+"_max", help)
	mean, _ := e.Gauges.Fetch(name+"_mean", help)
	stddev, _ := e.Gauges.Fetch(name+"_stddev", help)

	properties, _ := json.ChildrenMap()
	for key, property := range properties {
		switch key {
		case "mean_rate", "m1_rate", "m5_rate", "m15_rate":
			if value, ok := property.Data().(float64); ok {
				rates.WithLabelValues(renameRate(key)).Set(value)
			}

		case "p50", "p75", "p95", "p98", "p99", "p999":
			if value, ok := property.Data().(float64); ok {
				percentiles.WithLabelValues("0." + key[1:]).Set(value)
			}
		case "min":
			if value, ok := property.Data().(float64); ok {
				min.WithLabelValues().Set(value)
			}
		case "max":
			if value, ok := property.Data().(float64); ok {
				max.WithLabelValues().Set(value)
			}
		case "mean":
			if value, ok := property.Data().(float64); ok {
				mean.WithLabelValues().Set(value)
			}
		case "stddev":
			if value, ok := property.Data().(float64); ok {
				stddev.WithLabelValues().Set(value)
			}
		}
	}

	if new {
		log.Infof("Adding timer %s with initial count %v\n", name, count)
	}

	return []prometheus.Collector{counter, rates, percentiles, max, mean, min, stddev}, nil
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
