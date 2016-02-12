package main

import "testing"

func Test_container_key(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
		expect string
	}{
		{
			name:   "foo",
			labels: []string{},
			expect: "foo{}",
		}, {
			name:   "foo",
			labels: []string{"value"},
			expect: "foo{value}",
		}, {
			name:   "foo",
			labels: []string{"value", "color"},
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
	container.GetOrCreate("foo")

	if len(container.counters) != 1 {
		t.Fatalf("expected a counter, got %d counter(s)", len(container.counters))
	}

	container.GetOrCreate("foo")
	if len(container.counters) != 1 {
		t.Fatalf("expected same counter as before, go %d counter(s)", len(container.counters))
	}
}

func Test_get_or_create_gauge(t *testing.T) {
	container := NewGaugeContainer()
	container.GetOrCreate("foo")

	if len(container.gauges) != 1 {
		t.Fatalf("expected a gauge, got %d gauge(s)", len(container.gauges))
	}

	container.GetOrCreate("foo")
	if len(container.gauges) != 1 {
		t.Fatalf("expected same gauge as before, go %d gauge(s)", len(container.gauges))
	}
}
