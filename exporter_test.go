package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
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

type testExporter struct {
	exporter *Exporter
	server   *httptest.Server
}

func (s *testScraper) Scrape(path string) ([]byte, error) {
	return []byte(s.results), nil
}

func newTestExporter(namespace string) *testExporter {
	exporter := NewExporter(&testScraper{`{}`}, namespace)

	prometheus.MustRegister(exporter)
	server := httptest.NewServer(prometheus.UninstrumentedHandler())
	return &testExporter{
		exporter: exporter,
		server:   server,
	}
}

func (te *testExporter) close() {
	prometheus.Unregister(te.exporter)
	te.server.Close()
}

func (te *testExporter) export(json string) ([]byte, error) {

	te.exporter.scraper = &testScraper{json}
	response, err := http.Get(te.server.URL)
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

func getFunctionName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	parts := strings.Split(f.Name(), ".")
	return parts[len(parts)-1]
}

func export(json string) ([]byte, error) {
	exporter := NewExporter(&testScraper{json}, "marathon")
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

func assertResultsContain(t *testing.T, results []byte, patterns ...string) {
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if !re.Match(results) {
			t.Errorf("No metric matching pattern: %s\n", re)
		}
	}
}

func assertResultsDoNotContain(t *testing.T, results []byte, patterns ...string) {
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.Match(results) {
			t.Errorf("Metric matching pattern: '%s' should not exist\n", re)
		}
	}
}

func Test_export_counters(t *testing.T) {

	fName := getFunctionName()
	te := newTestExporter(fName)
	defer te.close()

	// First pass
	results, err := te.export(`{
		"counters": {
			"foo_count": {"count": 1},
			"bar_count": {"count": 2}
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_count 1",
		fName+"_bar_count 2")

	// Second pass; 'bar' metric no longer present
	results, err = te.export(`{
		"counters": {
			"foo_count": {"count": 1},
			"baz_count": {"count": 3}
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_count 1",
		fName+"_baz_count 3")

	assertResultsDoNotContain(t, results,
		fName+"_bar_count 2")
}

func Test_export_gauges(t *testing.T) {

	fName := getFunctionName()
	te := newTestExporter(fName)
	defer te.close()

	results, err := te.export(`{
		"gauges": {
			"foo_value": {"value": 1},
			"bar_value": {"value": 2}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_value 1",
		fName+"_bar_value 2")

	results, err = te.export(`{
		"gauges": {
			"foo_value": {"value": 1},
			"baz_value": {"value": 3}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_value 1",
		fName+"_baz_value 3")

	assertResultsDoNotContain(t, results,
		fName+"_bar_value 2")
}

func Test_export_meters(t *testing.T) {

	fName := getFunctionName()
	te := newTestExporter(fName)
	defer te.close()

	results, err := te.export(`{
		"meters": {
			"foo_meter": {"count":1,"m1_rate":1,"m5_rate":1,"m15_rate":1,"mean_rate":1,"units":"foos/bar"},
			"bar_meter": {"count":2,"m1_rate":2,"m5_rate":2,"m15_rate":2,"mean_rate":2,"units":"foos/bar"}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_meter_count 1",
		fName+"_foo_meter{rate=\"(1m|5m|15m|mean)\"} 1",
		fName+"_bar_meter_count 2",
		fName+"_bar_meter{rate=\"(1m|5m|15m|mean)\"} 2")

	results, err = te.export(`{
		"meters": {
			"foo_meter": {"count":1,"m1_rate":1,"m5_rate":1,"m15_rate":1,"mean_rate":1,"units":"foos/bar"},
			"baz_meter": {"count":2,"m1_rate":2,"m5_rate":2,"m15_rate":2,"mean_rate":2,"units":"foos/bar"}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_meter_count 1",
		fName+"_foo_meter{rate=\"(1m|5m|15m|mean)\"} 1",
		fName+"_baz_meter_count 2",
		fName+"_baz_meter{rate=\"(1m|5m|15m|mean)\"} 2")

	assertResultsDoNotContain(t, results,
		fName+"_bar_meter")
}

func Test_export_histograms(t *testing.T) {

	fName := getFunctionName()
	te := newTestExporter(fName)
	defer te.close()

	results, err := te.export(`{
		"histograms": {
			"foo_histogram": {"count":1,"p50":1,"p75":1,"p95":1,"p98":1,"p99":1,"p999":1,"max":1,"mean":1,"min":1,"stddev":1},
			"bar_histogram": {"count":2,"p50":2,"p75":2,"p95":2,"p98":2,"p99":2,"p999":2,"max":2,"mean":2,"min":2,"stddev":2}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_histogram_count 1",
		fName+"_foo_histogram_max 1",
		fName+"_foo_histogram_min 1",
		fName+"_foo_histogram_mean 1",
		fName+"_foo_histogram_stddev 1",
		fName+"_foo_histogram{percentile=\"0\\.\\d+\"} 1",
		fName+"_bar_histogram_count 2",
		fName+"_bar_histogram_max 2",
		fName+"_bar_histogram_min 2",
		fName+"_bar_histogram_mean 2",
		fName+"_bar_histogram_stddev 2",
		fName+"_bar_histogram{percentile=\"0\\.\\d+\"} 2")

	results, err = te.export(`{
		"histograms": {
			"foo_histogram": {"count":1,"p50":1,"p75":1,"p95":1,"p98":1,"p99":1,"p999":1,"max":1,"mean":1,"min":1,"stddev":1},
			"baz_histogram": {"count":2,"p50":2,"p75":2,"p95":2,"p98":2,"p99":2,"p999":2,"max":2,"mean":2,"min":2,"stddev":2}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_histogram_count 1",
		fName+"_foo_histogram_max 1",
		fName+"_foo_histogram_min 1",
		fName+"_foo_histogram_mean 1",
		fName+"_foo_histogram_stddev 1",
		fName+"_foo_histogram{percentile=\"0\\.\\d+\"} 1",
		fName+"_baz_histogram_count 2",
		fName+"_baz_histogram_max 2",
		fName+"_baz_histogram_min 2",
		fName+"_baz_histogram_mean 2",
		fName+"_baz_histogram_stddev 2",
		fName+"_baz_histogram{percentile=\"0\\.\\d+\"} 2")

	assertResultsDoNotContain(t, results,
		fName+"_bar_histogram")

}

func Test_export_timers(t *testing.T) {

	fName := getFunctionName()
	te := newTestExporter(fName)
	defer te.close()

	results, err := te.export(`{
		"timers": {
			"foo_timer": {"count":1,"p50":1,"p75":1,"p95":1,"p98":1,"p99":1,"p999":1,"max":1,"mean":1,"min":1,"stddev":1,"m1_rate":1,"m5_rate":1,"m15_rate":1,"mean_rate":1,"duration_units":"foos","rate_units":"bars/foo"},
			"bar_timer": {"count":2,"p50":2,"p75":2,"p95":2,"p98":2,"p99":2,"p999":2,"max":2,"mean":2,"min":2,"stddev":2,"m1_rate":2,"m5_rate":2,"m15_rate":2,"mean_rate":2,"duration_units":"bars","rate_units":"foos/bar"}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_timer_count 1",
		fName+"_foo_timer_max 1",
		fName+"_foo_timer_min 1",
		fName+"_foo_timer_mean 1",
		fName+"_foo_timer_stddev 1",
		fName+"_foo_timer{percentile=\"0\\.\\d+\"} 1",
		fName+"_foo_timer_rate{rate=\"(1m|5m|15m|mean)\"} 1",
		fName+"_bar_timer_count 2",
		fName+"_bar_timer_max 2",
		fName+"_bar_timer_min 2",
		fName+"_bar_timer_mean 2",
		fName+"_bar_timer_stddev 2",
		fName+"_bar_timer{percentile=\"0\\.\\d+\"} 2",
		fName+"_bar_timer_rate{rate=\"(1m|5m|15m|mean)\"} 2")

	results, err = te.export(`{
		"timers": {
			"foo_timer": {"count":1,"p50":1,"p75":1,"p95":1,"p98":1,"p99":1,"p999":1,"max":1,"mean":1,"min":1,"stddev":1,"m1_rate":1,"m5_rate":1,"m15_rate":1,"mean_rate":1,"duration_units":"foos","rate_units":"bars/foo"},
			"baz_timer": {"count":2,"p50":2,"p75":2,"p95":2,"p98":2,"p99":2,"p999":2,"max":2,"mean":2,"min":2,"stddev":2,"m1_rate":2,"m5_rate":2,"m15_rate":2,"mean_rate":2,"duration_units":"bars","rate_units":"foos/bar"}
		}
	}`)

	if err != nil {
		t.Fatal(err)
	}

	assertResultsContain(t, results,
		fName+"_foo_timer_count 1",
		fName+"_foo_timer_max 1",
		fName+"_foo_timer_min 1",
		fName+"_foo_timer_mean 1",
		fName+"_foo_timer_stddev 1",
		fName+"_foo_timer{percentile=\"0\\.\\d+\"} 1",
		fName+"_foo_timer_rate{rate=\"(1m|5m|15m|mean)\"} 1",
		fName+"_baz_timer_count 2",
		fName+"_baz_timer_max 2",
		fName+"_baz_timer_min 2",
		fName+"_baz_timer_mean 2",
		fName+"_baz_timer_stddev 2",
		fName+"_baz_timer{percentile=\"0\\.\\d+\"} 2",
		fName+"_baz_timer_rate{rate=\"(1m|5m|15m|mean)\"} 2")

	assertResultsDoNotContain(t, results,
		fName+"_bar_timer")
}
