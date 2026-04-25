package system

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

type SystemInfo struct {
	OS       string
	Kernel   string
	Shell    string
	ShellVer string
	CPU      string
	CPUCores string
	MemTotal string
	MemAvail string
	GPU      string
	Hostname string
	User     string
	WorkDir  string
	Locale   string
}

func Detect() SystemInfo {
	info := SystemInfo{
		OS:       detectOS(),
		Kernel:   run("uname", "-r"),
		Hostname: run("hostname"),
		WorkDir:  cwd(),
		User:     currentUser(),
	}
	info.Shell, info.ShellVer = detectShell()
	info.CPU, info.CPUCores = detectCPU()
	info.MemTotal, info.MemAvail = detectMemory()
	info.GPU = detectGPU()
	info.Locale = detectLocale()
	return info
}

func (s SystemInfo) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("OS: %s\n", s.OS))
	b.WriteString(fmt.Sprintf("Kernel: %s\n", s.Kernel))
	b.WriteString(fmt.Sprintf("Shell: %s %s\n", s.Shell, s.ShellVer))
	b.WriteString(fmt.Sprintf("CPU: %s (%s cores)\n", s.CPU, s.CPUCores))
	b.WriteString(fmt.Sprintf("Memory: %s total, %s available\n", s.MemTotal, s.MemAvail))
	if s.GPU != "" {
		b.WriteString(fmt.Sprintf("GPU: %s\n", s.GPU))
	}
	b.WriteString(fmt.Sprintf("Locale: %s\n", s.Locale))
	b.WriteString(fmt.Sprintf("Host: %s | User: %s\n", s.Hostname, s.User))
	b.WriteString(fmt.Sprintf("Working directory: %s\n", s.WorkDir))
	return b.String()
}

func (s SystemInfo) ContextBlock() string {
	return fmt.Sprintf("[System Context]\n%s", s.String())
}

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

func detectShell() (string, string) {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return "unknown", ""
	}
	shell := shellPath[strings.LastIndex(shellPath, "/")+1:]
	ver := run(shellPath, "--version")
	if idx := strings.IndexByte(ver, '\n'); idx > 0 {
		ver = ver[:idx]
	}
	return shell, ver
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

func detectLocale() string {
	// LANG is the primary locale indicator
	if lang := os.Getenv("LANG"); lang != "" {
		return lang
	}
	if lc := os.Getenv("LC_ALL"); lc != "" {
		return lc
	}
	if lc := os.Getenv("LC_MESSAGES"); lc != "" {
		return lc
	}
	// Fallback to locale command
	if out := run("locale"); out != "" {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "LANG=") {
				return strings.TrimPrefix(line, "LANG=")
			}
		}
	}
	return "en_US.UTF-8"
}

func run(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func currentUser() string {
	u, err := user.Current()
	if err != nil {
		return os.Getenv("USER")
	}
	return u.Username
}
