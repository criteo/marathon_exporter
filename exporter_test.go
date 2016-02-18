package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	flag.Set("log.level", "error")
}

type testScraper struct {
	results string
}

func (s *testScraper) Scrape() ([]byte, error) {
	return []byte(s.results), nil
}

func export(json string) ([]byte, error) {
	exporter := NewExporter(&testScraper{json})
	prometheus.MustRegister(exporter)
	defer prometheus.Unregister(exporter)

	server := httptest.NewServer(prometheus.UninstrumentedHandler())
	defer server.Close()

	response, err := http.Get(server.URL)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func Test_export_version(t *testing.T) {
	results, err := export(`{
		"version": "3.0.0"
	}`)

	if err != nil {
		t.Fatal(err)
	}

	if line := `marathon_metrics_version{version="3.0.0"} 1`; !strings.Contains(string(results), line) {
		t.Errorf("No metric matching: %s\n", line)
	}
}

func Test_export_counters(t *testing.T) {
	results, err := export(`{
		"counters": {
			"foo_count": {"count": 1},
			"bar_count": {"count": 2}
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}

	//t.Log(string(results))
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile("marathon_foo_count 1"),
		regexp.MustCompile("marathon_bar_count 2"),
	} {
		if !re.Match(results) {
			t.Errorf("No counter matching pattern: %s\n", re)
		}
	}
}

func Test_export_gauges(t *testing.T) {
	results, err := export(`{
		"gauges": {
			"foo_value": {"value": 1},
			"bar_value": {"value": 2}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	//t.Log(string(results))
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile("marathon_foo_value 1"),
		regexp.MustCompile("marathon_bar_value 2"),
	} {
		if !re.Match(results) {
			t.Errorf("No gauge matching pattern: %s\n", re)
		}
	}
}

func Test_export_meters(t *testing.T) {
	results, err := export(`{
		"meters": {
			"foo_meter": {"count":1,"m1_rate":1,"m5_rate":1,"m15_rate":1,"mean_rate":1,"units":"foos/bar"},
			"bar_meter": {"count":2,"m1_rate":2,"m5_rate":2,"m15_rate":2,"mean_rate":2,"units":"foos/bar"}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	//t.Log(string(results))
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile("marathon_foo_meter_count 1"),
		regexp.MustCompile("marathon_foo_meter{rate=\"(1m|5m|15m|mean)\"} 1"),
		regexp.MustCompile("marathon_bar_meter_count 2"),
		regexp.MustCompile("marathon_bar_meter{rate=\"(1m|5m|15m|mean)\"} 2"),
	} {
		if !re.Match(results) {
			t.Errorf("No meter metric matching pattern: %s\n", re)
		}
	}
}

func Test_export_histograms(t *testing.T) {
	results, err := export(`{
		"histograms": {
			"foo_histogram": {"count":1,"p50":1,"p75":1,"p95":1,"p98":1,"p99":1,"p999":1,"max":1,"mean":1,"min":1,"stddev":1},
			"bar_histogram": {"count":2,"p50":2,"p75":2,"p95":2,"p98":2,"p99":2,"p999":2,"max":2,"mean":2,"min":2,"stddev":2}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	//t.Log(string(results))
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile("marathon_foo_histogram_count 1"),
		regexp.MustCompile("marathon_foo_histogram_max 1"),
		regexp.MustCompile("marathon_foo_histogram_min 1"),
		regexp.MustCompile("marathon_foo_histogram_mean 1"),
		regexp.MustCompile("marathon_foo_histogram_stddev 1"),
		regexp.MustCompile("marathon_foo_histogram{percentile=\"0\\.\\d+\"} 1"),
		regexp.MustCompile("marathon_bar_histogram_count 2"),
		regexp.MustCompile("marathon_bar_histogram_max 2"),
		regexp.MustCompile("marathon_bar_histogram_min 2"),
		regexp.MustCompile("marathon_bar_histogram_mean 2"),
		regexp.MustCompile("marathon_bar_histogram_stddev 2"),
		regexp.MustCompile("marathon_bar_histogram{percentile=\"0\\.\\d+\"} 2"),
	} {
		if !re.Match(results) {
			t.Errorf("No histogram metric matching pattern: %s\n", re)
		}
	}
}

func Test_export_timers(t *testing.T) {
	results, err := export(`{
		"timers": {
			"foo_timer": {"count":1,"p50":1,"p75":1,"p95":1,"p98":1,"p99":1,"p999":1,"max":1,"mean":1,"min":1,"stddev":1,"m1_rate":1,"m5_rate":1,"m15_rate":1,"mean_rate":1,"duration_units":"foos","rate_units":"bars/foo"},
			"bar_timer": {"count":2,"p50":2,"p75":2,"p95":2,"p98":2,"p99":2,"p999":2,"max":2,"mean":2,"min":2,"stddev":2,"m1_rate":2,"m5_rate":2,"m15_rate":2,"mean_rate":2,"duration_units":"bars","rate_units":"foos/bar"}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	//t.Log(string(results))
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile("marathon_foo_timer_count 1"),
		regexp.MustCompile("marathon_foo_timer_max 1"),
		regexp.MustCompile("marathon_foo_timer_min 1"),
		regexp.MustCompile("marathon_foo_timer_mean 1"),
		regexp.MustCompile("marathon_foo_timer_stddev 1"),
		regexp.MustCompile("marathon_foo_timer{percentile=\"0\\.\\d+\"} 1"),
		regexp.MustCompile("marathon_foo_timer_rate{rate=\"(1m|5m|15m|mean)\"} 1"),
		regexp.MustCompile("marathon_bar_timer_count 2"),
		regexp.MustCompile("marathon_bar_timer_max 2"),
		regexp.MustCompile("marathon_bar_timer_min 2"),
		regexp.MustCompile("marathon_bar_timer_mean 2"),
		regexp.MustCompile("marathon_bar_timer_stddev 2"),
		regexp.MustCompile("marathon_bar_timer{percentile=\"0\\.\\d+\"} 2"),
		regexp.MustCompile("marathon_bar_timer_rate{rate=\"(1m|5m|15m|mean)\"} 2"),
	} {
		if !re.Match(results) {
			t.Errorf("No timer metric matching pattern: %s\n", re)
		}
	}
}
