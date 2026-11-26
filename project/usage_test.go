package main

import (
	"math"
	"testing"
)

func TestCPUUsageUsesIdleInsideKernelTime(t *testing.T) {
	usage, valid := cpuUsage(cpuTimes{100, 150, 50}, cpuTimes{160, 225, 75})
	if usage != 40 || !valid {
		t.Fatalf("got %d, %v; want 40%%", usage, valid)
	}
}

func TestCPUUsageDiscardsInvalidIntervals(t *testing.T) {
	start := cpuTimes{100, 200, 300}
	for _, end := range []cpuTimes{start, {99, 250, 320}, {150, 220, 320}} {
		if _, valid := cpuUsage(start, end); valid {
			t.Errorf("invalid sample accepted: %+v", end)
		}
	}
}

func TestGPUUsageSumsProcessesByEngineAndSelectsBusiest(t *testing.T) {
	samples := map[string]float64{
		`\GPU Engine(pid_55_luid_0x0_0x1_phys_0_eng_0_engtype_3D)\Utilization Percentage`:   31,
		`\GPU Engine(pid_99_luid_0x0_0x1_phys_0_eng_0_engtype_3D)\Utilization Percentage`:   25,
		`\GPU Engine(pid_55_luid_0x0_0x2_phys_0_eng_0_engtype_3D)\Utilization Percentage`:   49,
		`\GPU Engine(pid_55_luid_0x0_0x1_phys_0_eng_3_engtype_Copy)\Utilization Percentage`: 17,
		`\GPU Engine(pid_55_luid_0x0_0x1_phys_1_eng_0_engtype_3D)\Utilization Percentage`:   20,
	}
	usage := make(engineUsage)
	for path, value := range samples {
		if key := gpuEngineKey(path); key != "" {
			usage.add(key, value)
		}
	}
	if percentage, valid := usage.busiest(); percentage != 56 || !valid {
		t.Fatalf("got %d, %v; want 56%%", percentage, valid)
	}
}

func TestGPUUsageDistinguishesIdleFromUnavailable(t *testing.T) {
	usage := make(engineUsage)
	usage.add("engine", math.NaN())
	if _, valid := usage.busiest(); valid {
		t.Fatal("missing samples appeared valid")
	}
	usage.add("engine", 0)
	if value, valid := usage.busiest(); value != 0 || !valid {
		t.Fatalf("idle sample got %d, %v", value, valid)
	}
	usage.add("engine", 102)
	if value, valid := usage.busiest(); value != 100 || !valid {
		t.Fatalf("above-range sample got %d, %v", value, valid)
	}
	if gpuEngineKey("unsupported_instance") != "" {
		t.Fatal("accepted unknown counter name")
	}
}
