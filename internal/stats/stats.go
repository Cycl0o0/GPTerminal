package stats

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/usage"
)

type CommandStat struct {
	Name   string
	Count  int
	Cost   float64
	Tokens int
}

type DayEntry struct {
	Date  string
	Cost  float64
	Calls int
}

type Dashboard struct {
	Month       string
	TotalCost   float64
	TotalCalls  int
	TotalTokens int
	TopCommands []CommandStat
	DailyTrend  []DayEntry
}

func BuildDashboard(data usage.UsageData) Dashboard {
	d := Dashboard{
		Month:       data.Month,
		TotalCost:   data.TotalCost,
		TotalCalls:  data.Calls,
		TotalTokens: data.InputTokens + data.OutputTokens,
	}

	// Build top commands
	for name, cs := range data.CommandUsage {
		d.TopCommands = append(d.TopCommands, CommandStat{
			Name:   name,
			Count:  cs.Count,
			Cost:   cs.Cost,
			Tokens: cs.InputTokens + cs.OutputTokens,
		})
	}
	sort.Slice(d.TopCommands, func(i, j int) bool {
		return d.TopCommands[i].Count > d.TopCommands[j].Count
	})

	// Build daily trend (sorted by date)
	for date, cost := range data.DailyCosts {
		d.DailyTrend = append(d.DailyTrend, DayEntry{Date: date, Cost: cost})
	}
	sort.Slice(d.DailyTrend, func(i, j int) bool {
		return d.DailyTrend[i].Date < d.DailyTrend[j].Date
	})
	// Keep last 30 days
	if len(d.DailyTrend) > 30 {
		d.DailyTrend = d.DailyTrend[len(d.DailyTrend)-30:]
	}

	return d
}

func PrintPlain(d Dashboard) {
	fmt.Printf("GPTerminal Usage Dashboard — %s\n", d.Month)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total Cost:   $%.4f\n", d.TotalCost)
	fmt.Printf("Total Calls:  %d\n", d.TotalCalls)
	fmt.Printf("Total Tokens: %d\n", d.TotalTokens)
	fmt.Println()

	if len(d.TopCommands) > 0 {
		fmt.Println("Top Commands:")
		fmt.Printf("  %-16s %6s %10s %10s\n", "COMMAND", "CALLS", "COST", "TOKENS")
		fmt.Printf("  %s\n", strings.Repeat("─", 46))
		for _, cs := range d.TopCommands {
			fmt.Printf("  %-16s %6d %10s %10d\n", cs.Name, cs.Count, fmt.Sprintf("$%.4f", cs.Cost), cs.Tokens)
		}
		fmt.Println()
	}

	if len(d.DailyTrend) > 0 {
		fmt.Println("Daily Costs:")
		maxCost := 0.0
		for _, e := range d.DailyTrend {
			if e.Cost > maxCost {
				maxCost = e.Cost
			}
		}
		barWidth := 30
		for _, e := range d.DailyTrend {
			bars := 0
			if maxCost > 0 {
				bars = int((e.Cost / maxCost) * float64(barWidth))
			}
			if bars < 1 && e.Cost > 0 {
				bars = 1
			}
			fmt.Printf("  %s %s $%.4f\n", e.Date, strings.Repeat("█", bars), e.Cost)
		}
		fmt.Println()
	}
}
