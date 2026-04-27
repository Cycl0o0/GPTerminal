//go:build darwin

package system

import (
	"strings"
)

func detectOS() string {
	name := run("sw_vers", "-productName")
	version := run("sw_vers", "-productVersion")
	if name != "" {
		return strings.TrimSpace(name + " " + version)
	}
	return "macOS"
}

func detectCPU() (string, string) {
	model := run("sysctl", "-n", "machdep.cpu.brand_string")
	if model == "" {
		model = "unknown"
	}
	cores := run("sysctl", "-n", "hw.ncpu")
	if cores == "" {
		cores = "0"
	}
	return model, cores
}

func detectMemory() (string, string) {
	totalBytes := run("sysctl", "-n", "hw.memsize")
	if totalBytes == "" {
		return "unknown", "unknown"
	}
	return totalBytes + " bytes", "unknown"
}

func detectGPU() string {
	out := run("system_profiler", "SPDisplaysDataType")
	if out == "" {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Chipset Model:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "Chipset Model:"))
		}
	}
	return ""
}
