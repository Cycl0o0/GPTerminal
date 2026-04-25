package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func isOwnCommand(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	skipPrefixes := []string{"gpterminal", "fuck", "gptchat"}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// detectCurrentShell figures out the actual running shell by checking
// parent process, env vars, and $SHELL as fallback.
func detectCurrentShell() string {
	// Check FISH_VERSION — set by fish shell
	if os.Getenv("FISH_VERSION") != "" {
		return "fish"
	}
	// Check ZSH_VERSION — set by zsh
	if os.Getenv("ZSH_VERSION") != "" {
		return "zsh"
	}
	// Check BASH_VERSION — set by bash
	if os.Getenv("BASH_VERSION") != "" {
		return "bash"
	}

	// Try reading /proc/self/status or parent process
	ppidLink := fmt.Sprintf("/proc/%d/exe", os.Getppid())
	if target, err := os.Readlink(ppidLink); err == nil {
		base := filepath.Base(target)
		switch base {
		case "fish", "zsh", "bash":
			return base
		}
	}

	// Fallback to $SHELL
	return filepath.Base(os.Getenv("SHELL"))
}

func LastCommand() (string, error) {
	shellName := detectCurrentShell()

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch shellName {
	case "fish":
		return lastFishCommand(home)
	case "zsh":
		return lastLineBasedCommand(filepath.Join(home, ".zsh_history"), "zsh")
	default:
		return lastLineBasedCommand(filepath.Join(home, ".bash_history"), "bash")
	}
}

// lastFishCommand parses fish_history which uses a multi-line YAML-like format:
//
//	- cmd: some command
//	  when: 1234567890
func lastFishCommand(home string) (string, error) {
	histFile := filepath.Join(home, ".local", "share", "fish", "fish_history")
	data, err := os.ReadFile(histFile)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")

	// Walk backward looking for "- cmd: ..." lines
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "- cmd:") {
			continue
		}
		cmd := strings.TrimSpace(strings.TrimPrefix(line, "- cmd:"))
		if cmd == "" {
			continue
		}
		if isOwnCommand(cmd) {
			continue
		}
		return cmd, nil
	}
	return "", fmt.Errorf("no commands found in fish history")
}

// lastLineBasedCommand handles bash and zsh history (one command per line).
func lastLineBasedCommand(histFile, shell string) (string, error) {
	data, err := os.ReadFile(histFile)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// zsh extended history format: ": timestamp:0;command"
		if shell == "zsh" && strings.HasPrefix(line, ": ") {
			if idx := strings.Index(line, ";"); idx >= 0 {
				line = line[idx+1:]
			}
		}
		if line == "" {
			continue
		}
		if isOwnCommand(line) {
			continue
		}
		return line, nil
	}
	return "", fmt.Errorf("no commands found in %s history", shell)
}
