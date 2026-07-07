package code

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"golang.org/x/term"
)

const historyMax = 1000

// inputReader is a minimal raw-mode line editor: persistent history with
// Up/Down recall, backslash line continuation, Ctrl-C to cancel the current
// input, and Ctrl-D (on an empty line) to exit. It takes over the terminal
// only while reading one physical line, restoring cooked mode before
// returning so streaming assistant output is unaffected.
type inputReader struct {
	history     []string
	historyFile string
	out         io.Writer
}

func newInputReader() *inputReader {
	path := filepath.Join(config.ConfigDir(), "history_code")
	ir := &inputReader{historyFile: path, out: os.Stderr}
	if data, err := os.ReadFile(path); err == nil {
		var entries []string
		if json.Unmarshal(data, &entries) == nil {
			ir.history = entries
		}
	}
	return ir
}

func (ir *inputReader) saveHistory(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	if n := len(ir.history); n > 0 && ir.history[n-1] == line {
		return // don't record the same line twice in a row
	}
	ir.history = append(ir.history, line)
	if len(ir.history) > historyMax {
		ir.history = ir.history[len(ir.history)-historyMax:]
	}
	_ = os.MkdirAll(filepath.Dir(ir.historyFile), 0o700)
	if data, err := json.Marshal(ir.history); err == nil {
		_ = os.WriteFile(ir.historyFile, data, 0o600)
	}
}

const (
	readOK = iota
	readEOF
	readCancel // Ctrl-C: discard current input, return to a fresh prompt
)

// readLogicalLine reads one logical input, joining lines that end with a
// trailing backslash. Returns the assembled text and a status.
func (ir *inputReader) readLogicalLine(prompt string, contPrompt string) (string, int) {
	var text string
	p := prompt
	for {
		line, status := ir.readPhysicalLine(p)
		switch status {
		case readEOF:
			return "", readEOF
		case readCancel:
			return "", readCancel
		}
		if strings.HasSuffix(line, "\\") {
			text += strings.TrimSuffix(line, "\\") + "\n"
			p = contPrompt
			continue
		}
		text += line
		ir.saveHistory(text)
		return text, readOK
	}
}

func (ir *inputReader) readPhysicalLine(prompt string) (string, int) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Not a TTY (piped input): fall back to a plain blocking read.
		var buf [1]byte
		var line []byte
		for {
			n, e := os.Stdin.Read(buf[:])
			if n > 0 {
				if buf[0] == '\n' {
					return string(line), readOK
				}
				line = append(line, buf[0])
				continue
			}
			if e != nil {
				if len(line) == 0 {
					return "", readEOF
				}
				return string(line), readOK
			}
		}
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	fmt.Fprint(ir.out, prompt)
	var buf []byte
	histPos := len(ir.history) // len == "current" (not navigating)
	savedCurrent := ""

	redraw := func() {
		fmt.Fprint(ir.out, "\r\033[K") // CR + clear line
		fmt.Fprint(ir.out, prompt)
		ir.out.Write(buf)
	}

	for {
		var b [16]byte
		n, err := os.Stdin.Read(b[:])
		if err != nil && err != io.EOF {
			return "", readCancel
		}
		for i := 0; i < n; i++ {
			c := b[i]
			switch {
			case c == 0x03: // Ctrl-C
				fmt.Fprint(ir.out, "^C\r\n")
				return "", readCancel
			case c == 0x04: // Ctrl-D
				if len(buf) == 0 {
					fmt.Fprint(ir.out, "^D\r\n")
					return "", readEOF
				}
				// non-empty: ignore (forward-delete would move cursor; keep simple)
			case c == 0x7f || c == 0x08: // backspace
				if len(buf) > 0 {
					// drop one UTF-8 rune (handle multi-byte tails)
					buf = buf[:len(buf)-1]
					for len(buf) > 0 && buf[len(buf)-1] >= 0x80 && buf[len(buf)-1]&0xc0 == 0x80 {
						buf = buf[:len(buf)-1]
					}
					redraw()
				}
			case c == '\r' || c == '\n':
				fmt.Fprint(ir.out, "\r\n")
				return string(buf), readOK
			case c == 0x1b && i+2 < n && b[i+1] == '[':
				switch b[i+2] {
				case 'A': // up — older history
					if len(ir.history) > 0 && histPos > 0 {
						if histPos == len(ir.history) {
							savedCurrent = string(buf)
						}
						histPos--
						buf = []byte(ir.history[histPos])
						redraw()
					}
				case 'B': // down — newer history
					if histPos < len(ir.history) {
						histPos++
						if histPos == len(ir.history) {
							buf = []byte(savedCurrent)
						} else {
							buf = []byte(ir.history[histPos])
						}
						redraw()
					}
				default:
					// left/right/end/home: ignored (no cursor model)
				}
				i += 2
			case c < 0x20:
				// other control chars: ignore
			default:
				if histPos != len(ir.history) {
					// editing while viewing history: commit to current
					histPos = len(ir.history)
					savedCurrent = ""
				}
				buf = append(buf, c)
				// echo the byte (works for ASCII; UTF-8 prints in sequence)
				ir.out.Write([]byte{c})
			}
		}
		if err == io.EOF && n == 0 {
			return "", readEOF
		}
	}
}
