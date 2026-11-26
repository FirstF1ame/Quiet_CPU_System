package main

import (
	"math"
	"strings"
)

type cpuTimes struct {
	idle, kernel, user uint64
}

func cpuUsage(before, after cpuTimes) (int, bool) {
	if after.idle < before.idle || after.kernel < before.kernel || after.user < before.user {
		return 0, false
	}
	idle := after.idle - before.idle
	total := after.kernel - before.kernel + after.user - before.user
	if total == 0 || idle > total {
		return 0, false
	}
	return int(math.Round(100 * float64(total-idle) / float64(total))), true
}

// Counter paths contain one instance per process and physical GPU engine.
func gpuEngineKey(path string) string {
	_, instance, ok := strings.Cut(path, "_luid_")
	if !ok {
		return ""
	}
	key, _, ok := strings.Cut(instance, "_engtype_")
	if !ok || !strings.Contains(key, "_phys_") || !strings.Contains(key, "_eng_") {
		return ""
	}
	return key
}

type engineUsage map[string]float64

func (usage engineUsage) add(engine string, value float64) {
	if !math.IsNaN(value) && !math.IsInf(value, 0) {
		usage[engine] += max(0, value)
	}
}

func (usage engineUsage) busiest() (int, bool) {
	var highest float64
	for _, percentage := range usage {
		highest = max(percentage, highest)
	}
	return int(math.Round(min(100, highest))), len(usage) > 0
}

type usageValue struct {
	percentage int
	valid      bool
}

type usageSnapshot [3]usageValue
