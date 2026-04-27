//go:build windows

package system

import (
	"runtime"
	"strings"
)

func detectOS() string {
	out := run("cmd", "/C", "ver")
	if out != "" {
		// ver output often has blank lines; find the non-empty one
		for _, line := range strings.Split(out, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return runtime.GOOS
}

func detectCPU() (string, string) {
	model := parseWmicValue(run("wmic", "cpu", "get", "Name", "/value"))
	if model == "" {
		model = "unknown"
	}
	cores := parseWmicValue(run("wmic", "cpu", "get", "NumberOfLogicalProcessors", "/value"))
	if cores == "" {
		cores = "0"
	}
	return model, cores
}

func detectMemory() (string, string) {
	total := parseWmicValue(run("wmic", "OS", "get", "TotalVisibleMemorySize", "/value"))
	if total == "" {
		total = "unknown"
	} else {
		total += " kB"
	}
	avail := parseWmicValue(run("wmic", "OS", "get", "FreePhysicalMemory", "/value"))
	if avail == "" {
		avail = "unknown"
	} else {
		avail += " kB"
	}
	return total, avail
}

func detectGPU() string {
	return parseWmicValue(run("wmic", "path", "win32_VideoController", "get", "Name", "/value"))
}

// parseWmicValue extracts the value from wmic /value output like "Name=Intel Core i7"
func parseWmicValue(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, "="); idx >= 0 {
			return strings.TrimSpace(trimmed[idx+1:])
		}
	}
	return ""
}
