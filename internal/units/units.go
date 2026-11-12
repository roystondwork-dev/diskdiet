// Package units formats byte sizes and parses ages and Docker size strings.
package units

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Sizes use decimal units (1 GB = 1,000,000,000 bytes), matching Finder,
// Windows Explorer's disk labels and Docker.
var byteUnits = []string{"B", "kB", "MB", "GB", "TB", "PB"}

// FormatBytes renders n as a short human-readable size, e.g. "40.2 GB".
func FormatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	if n < 1000 {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	i := 0
	for v >= 999.95 && i < len(byteUnits)-1 {
		v /= 1000
		i++
	}
	return fmt.Sprintf("%.1f %s", v, byteUnits[i])
}

// ParseAge parses an age such as "30d", "2w", "12h" or "90m".
// Days (d) and weeks (w) are added to Go's duration units. "0" means no age.
func ParseAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, errors.New("empty age")
	}
	if s == "0" {
		return 0, nil
	}
	for suffix, unit := range map[string]time.Duration{"d": 24 * time.Hour, "w": 7 * 24 * time.Hour} {
		if num, ok := strings.CutSuffix(s, suffix); ok {
			f, err := strconv.ParseFloat(num, 64)
			if err != nil || f < 0 || math.IsInf(f, 0) || math.IsNaN(f) || f > 100*365 {
				return 0, fmt.Errorf("invalid age %q: use a number followed by d, w or h, e.g. 30d", s)
			}
			return time.Duration(f * float64(unit)), nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid age %q: use a number followed by d, w or h, e.g. 30d", s)
	}
	return d, nil
}

// FormatAge renders a duration the way a user would type it: "30d", "2w", "12h".
func FormatAge(d time.Duration) string {
	const day = 24 * time.Hour
	switch {
	case d <= 0:
		return "0"
	case d%(7*day) == 0:
		return fmt.Sprintf("%dw", d/(7*day))
	case d%day == 0:
		return fmt.Sprintf("%dd", d/day)
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	default:
		return d.String()
	}
}

var dockerUnits = map[string]float64{
	"b": 1, "kb": 1e3, "mb": 1e6, "gb": 1e9, "tb": 1e12, "pb": 1e15,
	"kib": 1 << 10, "mib": 1 << 20, "gib": 1 << 30, "tib": 1 << 40,
}

// ParseDockerSize parses the sizes Docker prints, e.g. "1.2GB", "512.5kB",
// "0B" or "3.4GB (50%)".
func ParseDockerSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if s == "" {
		return 0, errors.New("empty size")
	}
	i := strings.IndexFunc(s, func(r rune) bool { return (r < '0' || r > '9') && r != '.' })
	if i <= 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	mult, ok := dockerUnits[strings.ToLower(strings.TrimSpace(s[i:]))]
	if !ok {
		return 0, fmt.Errorf("invalid size unit in %q", s)
	}
	return int64(math.Round(num * mult)), nil
}
