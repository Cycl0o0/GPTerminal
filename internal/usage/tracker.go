package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/config"
)

// UsageData holds tracked API spend for the current month.
type UsageData struct {
	Month        string             `json:"month"`
	TotalCost    float64            `json:"total_cost"`
	InputTokens  int                `json:"input_tokens"`
	OutputTokens int                `json:"output_tokens"`
	Calls        int                `json:"calls"`
	ImageCost    float64            `json:"image_cost"`
	ImagesGen    int                `json:"images_generated"`
	DailyCosts   map[string]float64 `json:"daily_costs,omitempty"`
}

// Tracker persists usage data and enforces budget limits.
type Tracker struct {
	mu   sync.Mutex
	data UsageData
	path string
}

var (
	global     *Tracker
	globalOnce sync.Once
)

// Global returns the singleton tracker instance.
func Global() *Tracker {
	globalOnce.Do(func() {
		path := filepath.Join(config.ConfigDir(), "usage.json")
		global = &Tracker{path: path}
		global.load()
	})
	return global
}

func currentMonth() string {
	return time.Now().Format("2006-01")
}

func currentDay() string {
	return time.Now().Format("2006-01-02")
}

func (t *Tracker) load() {
	data, err := os.ReadFile(t.path)
	if err != nil {
		t.data = UsageData{Month: currentMonth()}
		return
	}
	if err := json.Unmarshal(data, &t.data); err != nil {
		t.data = UsageData{Month: currentMonth()}
		return
	}
	if t.data.Month != currentMonth() {
		t.data = UsageData{Month: currentMonth()}
	}
}

func (t *Tracker) save() error {
	dir := filepath.Dir(t.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(t.path, data, 0600)
}

// RecordUsage records token usage from a chat completion.
func (t *Tracker) RecordUsage(model string, inputTokens, outputTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.data.Month != currentMonth() {
		t.data = UsageData{Month: currentMonth()}
	}

	cost := CostForTokens(model, inputTokens, outputTokens)
	t.data.InputTokens += inputTokens
	t.data.OutputTokens += outputTokens
	t.data.TotalCost += cost
	t.data.Calls++
	if t.data.DailyCosts == nil {
		t.data.DailyCosts = map[string]float64{}
	}
	t.data.DailyCosts[currentDay()] += cost
	_ = t.save()
}

// RecordImageUsage records cost from image generation.
func (t *Tracker) RecordImageUsage(model, size string, n int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.data.Month != currentMonth() {
		t.data = UsageData{Month: currentMonth()}
	}

	cost := CostForImageGeneration(model, size, n)
	t.data.ImageCost += cost
	t.data.TotalCost += cost
	t.data.ImagesGen += n
	if t.data.DailyCosts == nil {
		t.data.DailyCosts = map[string]float64{}
	}
	t.data.DailyCosts[currentDay()] += cost
	_ = t.save()
}

// CheckBudget returns an error if the cost limit has been exceeded.
func (t *Tracker) CheckBudget() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	limit := config.CostLimit()
	if limit <= 0 {
		return nil
	}
	if t.data.TotalCost >= limit {
		return fmt.Errorf("monthly cost limit reached ($%.4f / $%.2f). Adjust with: gpterminal config set cost_limit <amount>", t.data.TotalCost, limit)
	}
	return nil
}

// WarnIfNeeded prints a warning if usage is above the warn threshold.
func (t *Tracker) WarnIfNeeded() {
	t.mu.Lock()
	defer t.mu.Unlock()

	limit := config.CostLimit()
	if limit <= 0 {
		return
	}
	threshold := config.WarnThreshold()
	pct := (t.data.TotalCost / limit) * 100
	if pct >= threshold {
		fmt.Fprintf(os.Stderr, "\033[1;33m⚠ Usage at %.0f%% of monthly limit ($%.4f / $%.2f)\033[0m\n", pct, t.data.TotalCost, limit)
	}
}

// CurrentUsage returns a copy of the current usage data.
func (t *Tracker) CurrentUsage() UsageData {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.data.Month != currentMonth() {
		t.data = UsageData{Month: currentMonth()}
	}
	return t.data
}
