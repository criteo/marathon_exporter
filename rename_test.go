package main

import "testing"

func Test_rename_metric(t *testing.T) {
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
		}, {
			name:   "foo(bar)",
			expect: "foo_bar",
		},
	}

	for _, c := range cases {
		name := renameMetric(c.name)
		if name != c.expect {
			t.Errorf("expected metric named %s, got %s", c.expect, name)
		}
	}
}

func Test_rename_rate(t *testing.T) {
	cases := []struct {
		name   string
		expect string
	}{
		{
			name:   "mean_rate",
			expect: "mean",
		}, {
			name:   "m1_rate",
			expect: "1m",
		}, {
			name:   "m5_rate",
			expect: "5m",
		}, {
			name:   "m15_rate",
			expect: "15m",
		}, {
			name:   "foo",
			expect: "foo",
		},
	}

	for _, c := range cases {
		name := renameRate(c.name)
		if name != c.expect {
			t.Errorf("expected rate named %s, got %s", c.expect, name)
		}
	}
}
