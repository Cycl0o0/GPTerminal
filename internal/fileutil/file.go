package fileutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func ReadText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("%s appears to be a binary file", path)
	}
	return string(data), nil
}

func WriteText(path, content string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
