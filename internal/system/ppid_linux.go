//go:build linux

package system

import (
	"fmt"
	"os"
	"path/filepath"
)

func ppidShell() string {
	ppidLink := fmt.Sprintf("/proc/%d/exe", os.Getppid())
	target, err := os.Readlink(ppidLink)
	if err != nil {
		return ""
	}
	base := filepath.Base(target)
	switch base {
	case "fish", "zsh", "bash":
		return base
	}
	return ""
}
