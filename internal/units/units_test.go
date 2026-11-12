package units

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		-5:             "0 B",
		0:              "0 B",
		999:            "999 B",
		1000:           "1.0 kB",
		1500:           "1.5 kB",
		999_949:        "999.9 kB",
		999_950:        "1.0 MB",
		40_200_000_000: "40.2 GB",
		3e15:           "3.0 PB",
		5e18:           "5000.0 PB",
	}
	for in, want := range cases {
		if got := FormatBytes(in); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAge(t *testing.T) {
	day := 24 * time.Hour
	ok := map[string]time.Duration{
		"30d":   30 * day,
		"2W":    14 * day,
		"1.5d":  36 * time.Hour,
		"12h":   12 * time.Hour,
		"90m":   90 * time.Minute,
		"0":     0,
		"0d":    0,
		"1h30m": 90 * time.Minute,
	}
	if got, err := ParseAge("  2w\t"); err != nil || got != 14*day {
		t.Errorf("ParseAge with spaces = %v, %v", got, err)
	}
	for in, want := range ok {
		got, err := ParseAge(in)
		if err != nil || got != want {
			t.Errorf("ParseAge(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "d", "-3d", "-1h", "xw", "40000d", "nand", "infd", "3y"} {
		if _, err := ParseAge(in); err == nil {
			t.Errorf("ParseAge(%q): want error", in)
		}
	}
}

func TestFormatAge(t *testing.T) {
	day := 24 * time.Hour
	cases := map[time.Duration]string{
		-time.Hour:                "0",
		0:                         "0",
		14 * day:                  "2w",
		30 * day:                  "30d",
		12 * time.Hour:            "12h",
		90 * time.Minute:          "1h30m0s",
		36 * time.Hour:            "36h",
		7*day + 0*time.Nanosecond: "1w",
	}
	for in, want := range cases {
		if got := FormatAge(in); got != want {
			t.Errorf("FormatAge(%v) = %q, want %q", in, got, want)
		}
	}
	// Round trip.
	for _, s := range []string{"30d", "2w", "12h"} {
		d, _ := ParseAge(s)
		if FormatAge(d) != s {
			t.Errorf("round trip %q -> %q", s, FormatAge(d))
		}
	}
}

func TestParseDockerSize(t *testing.T) {
	ok := map[string]int64{
		"0B":          0,
		"1.2GB":       1_200_000_000,
		"512.5kB":     512_500,
		"3.4GB (50%)": 3_400_000_000,
		" 10 MB ":     10_000_000,
		"1KiB":        1024,
		"2MiB":        2 << 20,
		"1GiB":        1 << 30,
		"1TiB":        1 << 40,
		"1TB":         1e12,
		"1PB":         1e15,
		"7b":          7,
	}
	for in, want := range ok {
		got, err := ParseDockerSize(in)
		if err != nil || got != want {
			t.Errorf("ParseDockerSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "(50%)", "GB", "1.2.3GB", "12XB", "12"} {
		if _, err := ParseDockerSize(in); err == nil {
			t.Errorf("ParseDockerSize(%q): want error", in)
		}
	}
}
