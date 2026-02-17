package agent

import (
	"fmt"
	"strconv"
	"strings"
)

// parseMemoryLimit parses a human-readable memory limit (e.g. "2g", "512m") to bytes.
func parseMemoryLimit(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" {
		return 0, nil
	}

	var multiplier int64
	switch {
	case strings.HasSuffix(s, "g"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "m"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "k"):
		multiplier = 1024
		s = strings.TrimSuffix(s, "k")
	default:
		multiplier = 1
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parseMemoryLimit(%q): %w", s, err)
	}

	return val * multiplier, nil
}

// parseCPULimit parses a CPU limit string (e.g. "2", "0.5") to Docker CPU quota.
// Docker uses 100000 microseconds per CPU period.
func parseCPULimit(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parseCPULimit(%q): %w", s, err)
	}

	// Docker CPU period is 100000 microseconds. Quota = cpus * period.
	const cpuPeriod = 100000
	return int64(val * cpuPeriod), nil
}
