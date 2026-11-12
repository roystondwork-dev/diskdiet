package units

import "testing"

func FuzzParseAge(f *testing.F) {
	for _, s := range []string{"30d", "2w", "12h", "0", "1.5d", "-1d", "99999999999d", "NaNd", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		d, err := ParseAge(s)
		if err == nil && d < 0 {
			t.Fatalf("ParseAge(%q) = %v, negative", s, d)
		}
	})
}

func FuzzParseDockerSize(f *testing.F) {
	for _, s := range []string{"1.2GB", "512.5kB", "0B", "3.4GB (50%)", "1e9GB", "GB", ""} {
		f.Add(s)
	}
	f.Fuzz(func(_ *testing.T, s string) {
		_, _ = ParseDockerSize(s) // must not panic
	})
}
