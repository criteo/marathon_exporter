package main

import (
	"flag"
	"net/url"
	"testing"

	"github.com/jeffail/gabs"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func init() {
	flag.Set("log.level", "error")
}

type scrapeResult struct {
	desc *prometheus.Desc
	data dto.Metric
}

func scrape(json string) []scrapeResult {
	var results []scrapeResult

	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			result := scrapeResult{desc: m.Desc()}
			m.Write(&result.data)
			results = append(results, result)
		}
		close(doneCh)
	}()

	content, _ := gabs.ParseJSON([]byte(json))
	exporter := NewExporter(&url.URL{})
	exporter.scrapeMetrics(content, metricCh)

	close(metricCh)
	<-doneCh

	return results
}

func desc(name, help string, labels ...string) string {
	return prometheus.NewDesc("marathon_"+name, help, labels, prometheus.Labels{}).String()
}

func Test_scrape_counters(t *testing.T) {
	results := scrape(`{
		"counters": {
			"foo": {"count": 123},
			"bar": {"count": 987}
		}
	}`)

	cases := map[string]string{
		desc("foo", "Marathon counter foo"): `counter:<value:123 > `,
		desc("bar", "Marathon counter bar"): `counter:<value:987 > `,
	}

	if len(cases) != len(results) {
		t.Fatalf("expected %d metrics, got %d", len(cases), len(results))
	}

	for _, result := range results {
		data, ok := cases[result.desc.String()]
		if !ok {
			t.Errorf("couldn't find counter matching desc: %s\n", result.desc.String())
		}
		if data != result.data.String() {
			t.Errorf("expected counter data %s, but got %v", data, result.data)
		}
	}
}

func Test_scrape_gauges(t *testing.T) {
	results := scrape(`{
		"gauges": {
			"foo": {"value": 123},
			"bar": {"value": 987}
		}
	}`)

	cases := map[string]string{
		desc("foo", "Marathon gauge foo"): `gauge:<value:123 > `,
		desc("bar", "Marathon gauge bar"): `gauge:<value:987 > `,
	}

	if len(cases) != len(results) {
		t.Fatalf("expected %d metrics, got %d", len(cases), len(results))
	}

	for _, result := range results {
		data, ok := cases[result.desc.String()]
		if !ok {
			t.Errorf("couldn't find gauge matching desc: %s\n", result.desc.String())
		}
		if data != result.data.String() {
			t.Errorf("expected gauge data %s, but got %v", data, result.data)
		}
	}
}

func Test_scrape_meters(t *testing.T) {
	results := scrape(`{
		"meters": {
			"foo": {"count":123,"m1_rate":1,"m15_rate":2,"m5_rate":3,"mean_rate":4,"units":"foos/bar"}
		}
	}`)

	cases := []struct {
		desc string
		data string
	}{
		{
			desc: desc("foo_count", "Marathon meter foo (foos/bar)"),
			data: `counter:<value:123 > `,
		}, {
			desc: desc("foo_rate", "Marathon meter foo (foos/bar)", "window"),
			data: `gauge:<value:1 > `,
		}, {
			desc: desc("foo_rate", "Marathon meter foo (foos/bar)", "window"),
			data: `gauge:<value:2 > `,
		}, {
			desc: desc("foo_rate", "Marathon meter foo (foos/bar)", "window"),
			data: `gauge:<value:3 > `,
		}, {
			desc: desc("foo_rate", "Marathon meter foo (foos/bar)", "window"),
			data: `gauge:<value:4 > `,
		},
	}

	if len(cases) != len(results) {
		t.Fatalf("expected %d metrics, got %d", len(cases), len(results))
	}

	for i, c := range cases {
		result := results[i]
		if c.desc != result.desc.String() {
			t.Errorf("expected meter desc %s, but got %v", c.desc, result.desc)
		}
		if c.desc != result.desc.String() {
			t.Errorf("expected meter data %s, but got %v", c.data, result.data)
		}
	}
}

func Test_scrape_histograms(t *testing.T) {
	results := scrape(`{
		"histograms": {
			"foo": {"count":123,"p50":1,"p75":2,"p95":3,"p98":4,"p99":5,"p999":6,"max":7,"mean":8,"min":9,"stddev":10}
		}
	}`)

	cases := []struct {
		desc string
		data string
	}{
		{
			desc: desc("foo_count", "Marathon histogram foo"),
			data: `counter:<value:123 > `,
		}, {
			desc: desc("foo", "Marathon histogram foo", "percentile"),
			data: `gauge:<value:1 > `,
		}, {
			desc: desc("foo", "Marathon histogram foo", "percentile"),
			data: `gauge:<value:2 > `,
		}, {
			desc: desc("foo", "Marathon histogram foo", "percentile"),
			data: `gauge:<value:3 > `,
		}, {
			desc: desc("foo", "Marathon histogram foo", "percentile"),
			data: `gauge:<value:4 > `,
		}, {
			desc: desc("foo", "Marathon histogram foo", "percentile"),
			data: `gauge:<value:5 > `,
		}, {
			desc: desc("foo", "Marathon histogram foo", "percentile"),
			data: `gauge:<value:6 > `,
		}, {
			desc: desc("foo_max", "Marathon histogram foo"),
			data: `gauge:<value:7 > `,
		}, {
			desc: desc("foo_mean", "Marathon histogram foo"),
			data: `gauge:<value:8 > `,
		}, {
			desc: desc("foo_min", "Marathon histogram foo"),
			data: `gauge:<value:9 > `,
		}, {
			desc: desc("foo_stddev", "Marathon histogram foo"),
			data: `gauge:<value:10 > `,
		},
	}

	if len(cases) != len(results) {
		t.Fatalf("expected %d metrics, got %d", len(cases), len(results))
	}

	for i, c := range cases {
		result := results[i]
		if c.desc != result.desc.String() {
			t.Errorf("expected histogram desc %s, but got %v", c.desc, result.desc)
		}
		if c.desc != result.desc.String() {
			t.Errorf("expected histogram data %s, but got %v", c.data, result.data)
		}
	}
}

func Test_scrape_timers(t *testing.T) {
	results := scrape(`{
		"timers": {
			"foo": {"count":123,"m1_rate":1,"m15_rate":2,"m5_rate":3,"mean_rate":4,"p50":1,"p75":2,"p95":3,"p98":4,"p99":5,"p999":6,"max":7,"mean":8,"min":9,"stddev":10,"duration_units":"bars","rate_units":"foos/bar"}
		}
	}`)

	cases := []struct {
		desc string
		data string
	}{
		{
			desc: desc("foo_count", "Marathon timer foo (foos/bar)"),
			data: `counter:<value:987 > `,
		}, {
			desc: desc("foo_rate", "Marathon timer foo (foos/bar)", "window"),
			data: `gauge:<value:1 > `,
		}, {
			desc: desc("foo_rate", "Marathon timer foo (foos/bar)", "window"),
			data: `gauge:<value:2 > `,
		}, {
			desc: desc("foo_rate", "Marathon timer foo (foos/bar)", "window"),
			data: `gauge:<value:3 > `,
		}, {
			desc: desc("foo_rate", "Marathon timer foo (foos/bar)", "window"),
			data: `gauge:<value:4 > `,
		}, {
			desc: desc("foo", "Marathon timer foo (foos/bar)", "percentile"),
			data: `gauge:<value:1 > `,
		}, {
			desc: desc("foo", "Marathon timer foo (foos/bar)", "percentile"),
			data: `gauge:<value:2 > `,
		}, {
			desc: desc("foo", "Marathon timer foo (foos/bar)", "percentile"),
			data: `gauge:<value:3 > `,
		}, {
			desc: desc("foo", "Marathon timer foo (foos/bar)", "percentile"),
			data: `gauge:<value:4 > `,
		}, {
			desc: desc("foo", "Marathon timer foo (foos/bar)", "percentile"),
			data: `gauge:<value:5 > `,
		}, {
			desc: desc("foo", "Marathon timer foo (foos/bar)", "percentile"),
			data: `gauge:<value:6 > `,
		}, {
			desc: desc("foo_max", "Marathon timer foo (foos/bar)"),
			data: `gauge:<value:7 > `,
		}, {
			desc: desc("foo_mean", "Marathon timer foo (foos/bar)"),
			data: `gauge:<value:8 > `,
		}, {
			desc: desc("foo_min", "Marathon timer foo (foos/bar)"),
			data: `gauge:<value:9 > `,
		}, {
			desc: desc("foo_stddev", "Marathon timer foo (foos/bar)"),
			data: `gauge:<value:10 > `,
		},
	}

	if len(cases) != len(results) {
		t.Fatalf("expected %d metrics, got %d", len(cases), len(results))
	}

	for i, c := range cases {
		result := results[i]
		if c.desc != result.desc.String() {
			t.Errorf("expected meter desc %s, but got %v", c.desc, result.desc)
		}
		if c.desc != result.desc.String() {
			t.Errorf("expected meter data %s, but got %v", c.data, result.data)
		}
	}
}
