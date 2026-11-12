package source

import (
	"context"
	"testing"
	"time"
)

func TestDemo(t *testing.T) {
	if DemoEnabled(func(string) string { return "" }) || DemoEnabled(func(string) string { return "0" }) ||
		!DemoEnabled(func(string) string { return "1" }) {
		t.Fatal("DemoEnabled")
	}
	ids := map[string]bool{}
	for _, id := range IDs() {
		ids[id] = true
	}
	for _, s := range Demo("/home/me") {
		if !ids[s.ID()] {
			t.Errorf("demo source %q is not a real source id", s.ID())
		}
		rm, all, week, quarter := s.Scan(context.Background(), month), s.Scan(context.Background(), every),
			s.Scan(context.Background(), Options{Age: 7 * 24 * time.Hour, Now: testNow}),
			s.Scan(context.Background(), Options{Age: 90 * 24 * time.Hour, Now: testNow})
		if !rm.Found || rm.Bytes == 0 || all.PrunableBytes < rm.PrunableBytes ||
			week.PrunableBytes < rm.PrunableBytes || quarter.PrunableBytes > rm.PrunableBytes || week.PrunableBytes > week.Bytes {
			t.Errorf("%s: month %d, all %d, week %d, quarter %d", s.ID(), rm.PrunableBytes, all.PrunableBytes, week.PrunableBytes, quarter.PrunableBytes)
		}
		res := s.Clean(context.Background(), rm, month)
		if res.Freed != rm.PrunableBytes {
			t.Errorf("%s: freed %d, want %d", s.ID(), res.Freed, rm.PrunableBytes)
		}
		if IsProjects(s.ID()) && len(Projects(rm, testNow)) != rm.PrunableItems {
			t.Errorf("%s: projects", s.ID())
		}
	}
}
