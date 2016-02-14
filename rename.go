package main

import "strings"

func renameRate(originalRate string) (name string) {
	switch originalRate {
	case "m1_rate":
		name = "1m"
	case "m5_rate":
		name = "5m"
	case "m15_rate":
		name = "15m"
	default:
		name = strings.TrimSuffix(originalRate, "_rate")
	}
	return
}

func renameMetric(originalName string) (name string) {
	name = strings.ToLower(originalName)
	name = strings.Replace(name, ".", "_", -1)
	name = strings.Replace(name, "-", "_", -1)
	name = strings.Replace(name, "$", "_", -1)
	name = strings.Replace(name, "(", "_", -1)
	name = strings.Replace(name, ")", "_", -1)
	name = strings.TrimRight(name, "_")
	return
}
