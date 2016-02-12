package main

import "testing"

func Test_metric_rename(t *testing.T) {
	cases := []struct {
		name   string
		expect string
	}{
		{
			name:   "Foo",
			expect: "foo",
		}, {
			name:   "foo_bar",
			expect: "foo_bar",
		}, {
			name:   "foo.bar",
			expect: "foo_bar",
		}, {
			name:   "foo-bar",
			expect: "foo_bar",
		}, {
			name:   "foo$bar",
			expect: "foo_bar",
		},
	}

	for _, c := range cases {
		name := metricName(c.name)
		if name != c.expect {
			t.Errorf("expected metric named %s, got %s", c.expect, name)
		}
	}
}
