package config

import (
	"fmt"
	"time"
)

// TimingEntry records a named timing measurement.
type TimingEntry struct {
	Name     string
	Duration time.Duration
}

// DebugTiming collects startup timing measurements for --debug output.
type DebugTiming struct {
	start   time.Time
	entries []TimingEntry
}

// NewDebugTiming creates a new timing collector.
func NewDebugTiming() *DebugTiming {
	return &DebugTiming{start: time.Now()}
}

// Record records a timing entry with an explicit duration.
func (dt *DebugTiming) Record(name string, d time.Duration) {
	dt.entries = append(dt.entries, TimingEntry{Name: name, Duration: d})
}

// Since records the elapsed time since start for a named step.
func (dt *DebugTiming) Since(name string, start time.Time) {
	dt.entries = append(dt.entries, TimingEntry{Name: name, Duration: time.Since(start)})
}

// Entries returns a copy of the recorded timing entries.
func (dt *DebugTiming) Entries() []TimingEntry {
	out := make([]TimingEntry, len(dt.entries))
	copy(out, dt.entries)
	return out
}

// Print prints the timing breakdown to stdout.
// Implements the --debug flag behavior from PRD §7.5.
func (dt *DebugTiming) Print() {
	total := time.Since(dt.start)

	fmt.Println("nidhi startup timing breakdown:")
	fmt.Println("================================")

	for _, e := range dt.entries {
		pct := float64(e.Duration) / float64(total) * 100
		fmt.Printf("  %-30s %8s  (%4.1f%%)\n", e.Name, e.Duration.Round(time.Microsecond), pct)
	}

	fmt.Printf("  %-30s %8s  (100%%)\n", "TOTAL", total.Round(time.Microsecond))
	fmt.Println()
	fmt.Printf("Interactive in: %s\n", total.Round(time.Millisecond))
}
