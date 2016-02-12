package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func Test_container_key(t *testing.T) {
	cases := []struct {
		name   string
		labels prometheus.Labels
		expect string
	}{
		{
			name:   "foo",
			labels: prometheus.Labels{},
			expect: "foo{}",
		}, {
			name: "foo",
			labels: prometheus.Labels{
				"value": "bar",
			},
			expect: "foo{value}",
		}, {
			name: "foo",
			labels: prometheus.Labels{
				"value": "bar",
				"color": "red",
			},
			expect: "foo{color,value}",
		},
	}

	for _, c := range cases {
		key := containerKey(c.name, c.labels)
		if key != c.expect {
			t.Errorf("expected container key %s, got %s", c.expect, key)
		}
	}
}

func Test_get_or_create_counter(t *testing.T) {
	container := NewCounterContainer()
	container.GetOrCreate("foo", prometheus.Labels{})

	if len(container.counters) != 1 {
		t.Fatalf("expected a counter, got %d counter(s)", len(container.counters))
	}

	container.GetOrCreate("foo", prometheus.Labels{})
	if len(container.counters) != 1 {
		t.Fatalf("expected same counter as before, go %d counter(s)", len(container.counters))
	}
}

func Test_get_or_create_gauge(t *testing.T) {
	container := NewGaugeContainer()
	container.GetOrCreate("foo", prometheus.Labels{})

	if len(container.gauges) != 1 {
		t.Fatalf("expected a gauge, got %d gauge(s)", len(container.gauges))
	}

	container.GetOrCreate("foo", prometheus.Labels{})
	if len(container.gauges) != 1 {
		t.Fatalf("expected same gauge as before, go %d gauge(s)", len(container.gauges))
	}
}
