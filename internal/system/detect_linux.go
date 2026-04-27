//go:build linux

package system

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

func detectOS() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	var name, version string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
		if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		}
		if strings.HasPrefix(line, "VERSION=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
		}
	}
	if name != "" {
		return strings.TrimSpace(name + " " + version)
	}
	return runtime.GOOS
}

func detectCPU() (string, string) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown", "0"
	}
	var model string
	cores := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") && model == "" {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				model = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "processor") {
			cores++
		}
	}
	if model == "" {
		model = "unknown"
	}
	return model, fmt.Sprintf("%d", cores)
}

func detectMemory() (string, string) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return "unknown", "unknown"
	}
	var total, avail string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			total = strings.TrimSpace(strings.TrimPrefix(line, "MemTotal:"))
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			avail = strings.TrimSpace(strings.TrimPrefix(line, "MemAvailable:"))
		}
	}
	return total, avail
}

func detectGPU() string {
	out := run("lspci")
	if out == "" {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(strings.ToLower(line), "vga") {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
			return strings.TrimSpace(line)
		}
	}
	return ""
}
