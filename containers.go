package main

import (
	"fmt"

	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	counterHelp   = "Marathon counter %s"
	gaugeHelp     = "Marathon gauge %s"
	meterHelp     = "Marathon meter %s (%s)"
	histogramHelp = "Marathon histogram %s"
	timerHelp     = "Marathon timer %s (%s)"
)

type CounterContainer struct {
	counters  map[string]*prometheus.CounterVec
	namespace string
}

func NewCounterContainer(namespace string) *CounterContainer {
	return &CounterContainer{
		counters:  make(map[string]*prometheus.CounterVec),
		namespace: namespace,
	}
}

func (c *CounterContainer) Fetch(name, help string, labels ...string) (*prometheus.CounterVec, bool) {
	key := containerKey(name, labels)
	counter, exists := c.counters[key]

	if !exists {
		counter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: c.namespace,
			Name:      name,
			Help:      help,
		}, labels)

		c.counters[key] = counter
	}

	return counter, !exists
}

type GaugeContainer struct {
	gauges    map[string]*prometheus.GaugeVec
	namespace string
}

func NewGaugeContainer(namespace string) *GaugeContainer {
	return &GaugeContainer{
		gauges:    make(map[string]*prometheus.GaugeVec),
		namespace: namespace,
	}
}

func (c *GaugeContainer) Fetch(name, help string, labels ...string) (*prometheus.GaugeVec, bool) {
	key := containerKey(name, labels)
	gauge, exists := c.gauges[key]

	if !exists {
		gauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: c.namespace,
			Name:      name,
			Help:      help,
		}, labels)

		c.gauges[key] = gauge
	}
	return gauge, !exists
}

func containerKey(metric string, labels []string) string {
	s := make([]string, len(labels))
	copy(s, labels)
	sort.Strings(s)
	return fmt.Sprintf("%s{%v}", metric, strings.Join(s, ","))
}
