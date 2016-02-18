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

func Test_container_fetch_counter(t *testing.T) {
	container := NewCounterContainer()
	_, new := container.Fetch("foo", "")

	if !new {
		t.Fatal("expected a new counter")
	}
	if len(container.counters) != 1 {
		t.Fatalf("expected a counter, got %d counters", len(container.counters))
	}

	_, new = container.Fetch("foo", "")
	if new {
		t.Fatal("expected an existing counter")
	}
	if len(container.counters) != 1 {
		t.Fatalf("expected same counter as before, go %d counters", len(container.counters))
	}
}

func Test_container_fetch_gauge(t *testing.T) {
	container := NewGaugeContainer()
	_, new := container.Fetch("foo", "")

	if !new {
		t.Fatal("expected a new gauge")
	}
	if len(container.gauges) != 1 {
		t.Fatalf("expected a gauge, got %d gauges", len(container.gauges))
	}

	_, new = container.Fetch("foo", "")
	if new {
		t.Fatal("expected an existing gauge")
	}
	if len(container.gauges) != 1 {
		t.Fatalf("expected same gauge as before, go %d gauges", len(container.gauges))
	}
}
