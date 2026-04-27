//go:build !linux

package system

func ppidShell() string {
	return ""
}
