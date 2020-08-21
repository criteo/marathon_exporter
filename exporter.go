package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jeffail/gabs"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const defaultNamespace = "marathon"

type Exporter struct {
	scraper      Scraper
	duration     prometheus.Gauge
	scrapeError  prometheus.Gauge
	up           prometheus.Gauge
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
	ch <- e.up
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()

	var err error
	defer func(begin time.Time) {
		e.duration.Set(time.Since(begin).Seconds())
		if err == nil {
			e.scrapeError.Set(0)
			e.up.Set(1)
		} else {
			e.totalErrors.Inc()
			e.scrapeError.Set(1)
			e.up.Set(0)
		}
	}(time.Now())

	// Rebuild gauges & coutners to avoid stale values
	e.Gauges = NewGaugeContainer(e.Gauges.namespace)
	e.Counters = NewCounterContainer(e.Counters.namespace)
	if err = e.exportApps(ch); err != nil {
		return
	}
	if err = e.exportMetrics(ch); err != nil {
		return
	}

	e.Counters.mutex.Lock()
	defer e.Counters.mutex.Unlock()
	for _, counter := range e.Counters.counters {
		counter.Collect(ch)
	}

	e.Gauges.mutex.Lock()
	defer e.Gauges.mutex.Unlock()
	for _, gauge := range e.Gauges.gauges {
		gauge.Collect(ch)
	}
}

func (e *Exporter) exportApps(ch chan<- prometheus.Metric) (err error) {
	content, err := e.scraper.Scrape("v2/apps?embed=apps.taskStats")
	if err != nil {
		log.Debugf("Problem scraping v2/apps endpoint: %v\n", err)
		return
	}

	json, err := gabs.ParseJSON(content)
	if err != nil {
		log.Debugf("Problem parsing v2/apps response: %v\n", err)
		return
	}

	e.scrapeApps(json, ch)
	return
}

func (e *Exporter) exportMetrics(ch chan<- prometheus.Metric) (err error) {
	content, err := e.scraper.Scrape("metrics")
	if err != nil {
		log.Debugf("Problem scraping metrics endpoint: %v\n", err)
		return
	}

	json, err := gabs.ParseJSON(content)
	if err != nil {
		log.Debugf("Problem parsing metrics response: %v\n", err)
		return
	}

	e.scrapeMetrics(json, ch)
	return
}

func (e *Exporter) scrapeApps(json *gabs.Container, ch chan<- prometheus.Metric) {
	elements, _ := json.S("apps").Children()
	states := map[string]string{
		"running":    "tasksRunning",
		"staged":     "tasksStaged",
		"healthy":    "tasksHealthy",
		"unhealthy":  "tasksUnhealthy",
		"cpus":       "cpus",
		"mem_in_mb":  "mem",
		"disk_in_mb": "disk",
		"gpus":       "gpus",
		"avg_uptime": "taskStats.startedAfterLastScaling.stats.lifeTime.averageSeconds",
	}

	name := "app_instances"
	gauge, new := e.Gauges.Fetch(name, "Marathon app instance count", "app", "app_version")
	if new {
		log.Infof("Added gauge %q\n", name)
	}
	gauge.Reset()

	for _, app := range elements {
		id := app.Path("id").Data().(string)
		version := app.Path("version").Data().(string)
		data := app.Path("instances").Data()
		count, ok := data.(float64)
		if !ok {
			log.Debugf(fmt.Sprintf("Bad conversion! Unexpected value \"%v\" for number of app instances\n", data))
			continue
		}

		gauge.WithLabelValues(id, version).Set(count)

		for key, value := range states {
			name := fmt.Sprintf("app_task_%s", key)
			gauge, new := e.Gauges.Fetch(name, fmt.Sprintf("Marathon app task %s count", key), "app", "app_version")
			if new {
				log.Infof("Added gauge %q\n", name)
			}

			data := app.Path(value).Data()
			count, ok := data.(float64)
			if !ok {
				log.Debugf(fmt.Sprintf("Bad conversion! Unexpected value \"%v\" for number of \"%s\" tasks\n", data, key))
				continue
			}

			gauge.WithLabelValues(id, version).Set(count)
		}
	}
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
			e.scrapeCounters(element)
		case "gauges":
			e.scrapeGauges(element)
		case "histograms":
			e.scrapeHistograms(element)
		case "meters":
			e.scrapeMeters(element)
		case "timers":
			e.scrapeTimers(element)
		}
	}
}

func (e *Exporter) scrapeCounters(json *gabs.Container) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		new, err := e.scrapeCounter(key, element)
		if err != nil {
			log.Debug(err)
		} else if new {
			log.Infof("Added counter %q\n", key)
		}
	}
}

func (e *Exporter) scrapeCounter(key string, json *gabs.Container) (bool, error) {
	data := json.Path("count").Data()
	count, ok := data.(float64)
	if !ok {
		return false, errors.New(fmt.Sprintf("Bad conversion! Unexpected value \"%v\" for counter %s\n", data, key))
	}

	name := renameMetric(key)
	help := fmt.Sprintf(counterHelp, key)
	counter, new := e.Counters.Fetch(name, help)
	counter.WithLabelValues().Set(count)
	return new, nil
}

func (e *Exporter) scrapeGauges(json *gabs.Container) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		new, err := e.scrapeGauge(key, element)
		if err != nil {
			log.Debug(err)
		} else if new {
			log.Infof("Added gauge %q\n", key)
		}
	}
}

func (e *Exporter) scrapeGauge(key string, json *gabs.Container) (bool, error) {
	value, ok := json.Path("value").Data().(float64)
	if !ok {
		// Let's try to scrap old min,max metric
		value, ok = json.Path("max").Data().(float64)
		if !ok {
			return false, errors.New(fmt.Sprintf("Bad conversion! Unexpected value for 'value' or 'max' in gauge %s\n", key))
		}
	}

	name := renameMetric(key)
	help := fmt.Sprintf(gaugeHelp, key)
	gauge, new := e.Gauges.Fetch(name, help)
	gauge.WithLabelValues().Set(value)
	return new, nil
}

func (e *Exporter) scrapeMeters(json *gabs.Container) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		new, err := e.scrapeMeter(key, element)
		if err != nil {
			log.Debug(err)
		} else if new {
			log.Infof("Added meter %q\n", key)
		}
	}
}

func (e *Exporter) scrapeMeter(key string, json *gabs.Container) (bool, error) {
	count, ok := json.Path("count").Data().(float64)
	if !ok {
		return false, errors.New(fmt.Sprintf("Bad meter! %s has no count\n", key))
	}
	units, ok := json.Path("units").Data().(string)
	if !ok {
		return false, errors.New(fmt.Sprintf("Bad meter! %s has no units\n", key))
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

	return new, nil
}

func (e *Exporter) scrapeHistograms(json *gabs.Container) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		new, err := e.scrapeHistogram(key, element)
		if err != nil {
			log.Debug(err)
		} else if new {
			log.Infof("Added histogram %q\n", key)
		}
	}
}

func (e *Exporter) scrapeHistogram(key string, json *gabs.Container) (bool, error) {
	count, ok := json.Path("count").Data().(float64)
	if !ok {
		return false, errors.New(fmt.Sprintf("Bad historgram! %s has no count\n", key))
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

	return new, nil
}

func (e *Exporter) scrapeTimers(json *gabs.Container) {
	elements, _ := json.ChildrenMap()
	for key, element := range elements {
		new, err := e.scrapeTimer(key, element)
		if err != nil {
			log.Debug(err)
		} else if new {
			log.Infof("Added timer %q\n", key)
		}
	}
}

func (e *Exporter) scrapeTimer(key string, json *gabs.Container) (bool, error) {
	count, ok := json.Path("count").Data().(float64)
	if !ok {
		return false, errors.New(fmt.Sprintf("Bad timer! %s has no count\n", key))
	}
	units, ok := json.Path("rate_units").Data().(string)
	if !ok {
		return false, errors.New(fmt.Sprintf("Bad timer! %s has no units\n", key))
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

	return new, nil
}

func NewExporter(s Scraper, namespace string) *Exporter {
	return &Exporter{
		scraper:  s,
		Counters: NewCounterContainer(namespace),
		Gauges:   NewGaugeContainer(namespace),
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from Marathon.",
		}),
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the last scrape of metrics from Marathon resulted in an error (0 for error, 1 for success).",
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
