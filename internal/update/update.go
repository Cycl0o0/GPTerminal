package update

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const repoAPI = "https://api.github.com/repos/Cycl0o0/GPTerminal/releases/latest"

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
	Body    string  `json:"body"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type CheckResult struct {
	Current     string
	Latest      string
	Available   bool
	Notes       string
	DownloadURL string
	Archive     bool
}

func Check(currentVersion string) (*CheckResult, error) {
	resp, err := http.Get(repoAPI)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	result := &CheckResult{
		Current: currentVersion,
		Latest:  latest,
		Notes:   rel.Body,
	}

	if CompareVersions(currentVersion, latest) < 0 {
		result.Available = true

		// Try .tar.gz archive first, then bare binary
		tarName := fmt.Sprintf("gpterminal-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
		bareName := fmt.Sprintf("gpterminal-%s-%s", runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			bareName += ".exe"
		}
		for _, a := range rel.Assets {
			if a.Name == tarName {
				result.DownloadURL = a.BrowserDownloadURL
				result.Archive = true
				break
			}
			if a.Name == bareName {
				result.DownloadURL = a.BrowserDownloadURL
				break
			}
		}
	}

	return result, nil
}

func Apply(result *CheckResult) error {
	if result.DownloadURL == "" {
		return fmt.Errorf("no download URL available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	resp, err := http.Get(result.DownloadURL)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	var binaryReader io.Reader
	if result.Archive {
		binaryReader, err = extractBinaryFromTarGz(resp.Body)
		if err != nil {
			return fmt.Errorf("extract archive: %w", err)
		}
	} else {
		binaryReader = resp.Body
	}

	tmpPath := execPath + ".new"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, binaryReader); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write update: %w", err)
	}
	tmpFile.Close()

	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	return os.Chmod(execPath, 0755)
}

// extractBinaryFromTarGz reads a .tar.gz stream and returns a reader for the
// first file whose name starts with "gpterminal".
func extractBinaryFromTarGz(r io.Reader) (io.Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		name := filepath.Base(hdr.Name)
		if hdr.Typeflag == tar.TypeReg && strings.HasPrefix(name, "gpterminal") {
			return tr, nil
		}
	}
	return nil, fmt.Errorf("no gpterminal binary found in archive")
}

func CompareVersions(a, b string) int {
	partsA := strings.Split(strings.TrimPrefix(a, "v"), ".")
	partsB := strings.Split(strings.TrimPrefix(b, "v"), ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			numA, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			numB, _ = strconv.Atoi(partsB[i])
		}
		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
	}
	return 0
}
