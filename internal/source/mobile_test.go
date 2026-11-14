package source

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAndroid(t *testing.T) {
	home := t.TempDir()
	sdk := filepath.Join(home, "Library", "Android", "sdk")
	img := func(api, tag, abi string) string { return filepath.Join(sdk, "system-images", api, tag, abi) }
	used, unused, unusedNew := img("android-34", "google_apis", "arm64-v8a"), img("android-29", "default", "x86_64"), img("android-35", "google_apis", "arm64-v8a")
	for _, p := range []string{used, unused} {
		mkOld(t, filepath.Join(p, "system.img"))
		age(t, p, oldTime)
	}
	mkNew(t, filepath.Join(unusedNew, "system.img"))
	// An emulator uses the android-34 image.
	writeFile(t, filepath.Join(home, ".android", "avd", "Pixel.avd", "config.ini"),
		"hw.lcd.density=420\nimage.sysdir.1=system-images/android-34/google_apis/arm64-v8a/\n")
	for _, v := range []string{"25.2.9519653", "26.1.10909125"} {
		mkOld(t, filepath.Join(sdk, "ndk", v, "source.properties"))
		age(t, filepath.Join(sdk, "ndk", v), oldTime)
	}
	for _, v := range []string{"33.0.2", "34.0.0"} {
		mkOld(t, filepath.Join(sdk, "build-tools", v, "aapt"))
		age(t, filepath.Join(sdk, "build-tools", v), oldTime)
	}

	s := NewAndroid(testEnv(home, "darwin", nil, nil))
	r := scanClean(t, s, 7, 3)
	if len(r.Locations) != 1 || r.Locations[0] != sdk {
		t.Errorf("locations = %v", r.Locations)
	}
	switch {
	case !exists(used):
		t.Error("an image an emulator uses was removed")
	case exists(unused):
		t.Error("an old unused image was kept")
	case !exists(unusedNew):
		t.Error("a recent image was removed")
	case !exists(filepath.Join(sdk, "ndk", "26.1.10909125")) || exists(filepath.Join(sdk, "ndk", "25.2.9519653")):
		t.Error("the newest NDK must stay and older ones go")
	case !exists(filepath.Join(sdk, "build-tools", "34.0.0")) || exists(filepath.Join(sdk, "build-tools", "33.0.2")):
		t.Error("the newest build tools must stay and older ones go")
	}

	// Locations from the environment and per OS.
	for _, tc := range []struct {
		goos string
		vars map[string]string
		want string
	}{
		{"linux", nil, filepath.Join(home, "Android", "Sdk")},
		{"windows", nil, filepath.Join(home, "AppData", "Local", "Android", "Sdk")},
		{"linux", map[string]string{"ANDROID_SDK_ROOT": "/sdk"}, "/sdk"},
		{"linux", map[string]string{"ANDROID_HOME": "/h", "ANDROID_SDK_ROOT": "/r"}, "/h"},
	} {
		if got := androidSDK(testEnv(home, tc.goos, tc.vars, nil)); got != tc.want {
			t.Errorf("%s %v: %s, want %s", tc.goos, tc.vars, got, tc.want)
		}
	}
	if got := avdHome(testEnv(home, "linux", map[string]string{"ANDROID_AVD_HOME": "/avd"}, nil)); got != "/avd" {
		t.Error(got)
	}
	if got := avdHome(testEnv(home, "linux", map[string]string{"ANDROID_USER_HOME": "/u"}, nil)); got != filepath.Join("/u", "avd") {
		t.Error(got)
	}
}

func TestHomebrew(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	cache := filepath.Join(home, "brewcache")
	oldDl := filepath.Join(cache, "downloads", "abc--wget-1.21.bottle.tar.gz")
	newDl := filepath.Join(cache, "downloads", "def--jq-1.7.bottle.tar.gz")
	mkOld(t, oldDl)
	mkNew(t, newDl)
	if runtime.GOOS != "windows" {
		if err := os.Symlink(oldDl, filepath.Join(cache, "wget--1.21.bottle.tar.gz")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(newDl, filepath.Join(cache, "jq--1.7.bottle.tar.gz")); err != nil {
			t.Fatal(err)
		}
	}
	r := &fakeRunner{installed: map[string]bool{"brew": true}, out: map[string]string{"brew --cache": cache + "\n"}}
	s := NewHomebrew(testEnv(home, "darwin", nil, r))
	if s.ID() != "homebrew" {
		t.Fatal("id")
	}
	rep := s.Scan(ctx, month)
	if rep.Items != 2 || rep.PrunableItems != 1 {
		t.Fatalf("report = %+v", rep)
	}
	res := s.Clean(ctx, rep, month)
	if res.Removed != 1 || exists(oldDl) || !exists(newDl) {
		t.Fatalf("result = %+v", res)
	}
	if runtime.GOOS != "windows" {
		if exists(filepath.Join(cache, "wget--1.21.bottle.tar.gz")) {
			t.Error("the shortcut to a removed download should go")
		}
		if !exists(filepath.Join(cache, "jq--1.7.bottle.tar.gz")) {
			t.Error("the shortcut to a kept download must stay")
		}
	}
	// Defaults and HOMEBREW_CACHE.
	hs := func(goos string, vars map[string]string) string {
		return NewHomebrew(testEnv(home, goos, vars, nil)).(*homebrewSource).cacheDir(ctx)
	}
	if got := hs("darwin", nil); got != filepath.Join(home, "Library", "Caches", "Homebrew") {
		t.Error(got)
	}
	if got := hs("linux", nil); got != filepath.Join(home, ".cache", "Homebrew") {
		t.Error(got)
	}
	if got := hs("linux", map[string]string{"HOMEBREW_CACHE": "/hc"}); got != "/hc" {
		t.Error(got)
	}
	removeDanglingLinks(filepath.Join(home, "missing")) // no folder: nothing to do
}

func TestCocoaPodsAndSwiftPM(t *testing.T) {
	home := t.TempDir()
	pods := filepath.Join(home, "Library", "Caches", "CocoaPods")
	mkOld(t, filepath.Join(pods, "Pods", "Release", "Alamofire", "5.6.0-abc", "Source.swift"))
	age(t, filepath.Join(pods, "Pods", "Release", "Alamofire", "5.6.0-abc"), oldTime)
	mkNew(t, filepath.Join(pods, "Pods", "Release", "Alamofire", "5.9.0-def", "Source.swift"))
	mkOld(t, filepath.Join(pods, "Pods", "External", "MyLib", "abc123", "x.m"))
	age(t, filepath.Join(pods, "Pods", "External", "MyLib", "abc123"), oldTime)
	r := scanClean(t, NewCocoaPods(testEnv(home, "darwin", nil, nil)), 3, 2)
	if r.Locations[0] != pods {
		t.Errorf("locations = %v", r.Locations)
	}
	if !exists(filepath.Join(pods, "Pods", "Release", "Alamofire", "5.9.0-def")) {
		t.Error("recent pod removed")
	}

	spm := filepath.Join(home, "Library", "Caches", "org.swift.swiftpm")
	mkOld(t, filepath.Join(spm, "repositories", "swift-nio-abc", "HEAD"))
	age(t, filepath.Join(spm, "repositories", "swift-nio-abc"), oldTime)
	mkNew(t, filepath.Join(spm, "artifacts", "big-xcframework", "x"))
	scanClean(t, NewSwiftPM(testEnv(home, "darwin", nil, nil)), 2, 1)

	if NewCocoaPods(testEnv(home, "windows", nil, nil)).(*dirSource).locate(context.Background()) != nil ||
		NewSwiftPM(testEnv(home, "windows", nil, nil)).(*dirSource).locate(context.Background()) != nil {
		t.Error("CocoaPods and SwiftPM have no Windows cache")
	}
}

func TestSimulators(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	core := filepath.Join(home, "Library", "Developer", "CoreSimulator")
	gone := filepath.Join(core, "Devices", "AAAA-1111")
	alive := filepath.Join(core, "Devices", "BBBB-2222")
	mkNew(t, filepath.Join(gone, "data", "big.img"))
	mkNew(t, filepath.Join(alive, "data", "big.img"))
	mkOld(t, filepath.Join(core, "Caches", "dyld", "x"))
	age(t, filepath.Join(core, "Caches", "dyld"), oldTime)
	list := map[string]any{"devices": map[string]any{
		"com.apple.CoreSimulator.SimRuntime.iOS-16-0": []map[string]any{
			{"udid": "AAAA-1111", "name": "iPhone 14", "isAvailable": false, "dataPath": filepath.Join(gone, "data")},
			{"udid": "EVIL", "name": "x", "isAvailable": false, "dataPath": filepath.Join(home, "elsewhere", "data")},
		},
		"com.apple.CoreSimulator.SimRuntime.iOS-17-0": []map[string]any{
			{"udid": "BBBB-2222", "name": "iPhone 15", "isAvailable": true, "dataPath": filepath.Join(alive, "data")},
		},
	}}
	out, _ := json.Marshal(list)
	deleted := false
	r := &fakeRunner{installed: map[string]bool{"xcrun": true}, handler: func(cmd string) ([]byte, error, bool) {
		switch cmd {
		case "xcrun simctl list devices unavailable -j":
			return out, nil, true
		case "xcrun simctl delete unavailable":
			deleted = true
			_ = os.RemoveAll(gone)
			return nil, nil, true
		}
		return nil, nil, false
	}}
	s := NewSimulators(testEnv(home, "darwin", nil, r))
	if s.ID() != "simulators" || s.Name() != "Xcode simulators" {
		t.Fatal("id")
	}
	rep := s.Scan(ctx, month)
	// The unavailable device (the one outside Devices is ignored) and the old cache.
	if !rep.Found || rep.PrunableItems != 2 || len(rep.Locations) != 1 || rep.Locations[0] != core || !hasNote(rep, "whatever their age") {
		t.Fatalf("report = %+v", rep)
	}
	res := s.Clean(ctx, rep, month)
	if !deleted || res.Freed <= 0 || res.Removed != 2 || len(res.Notes) != 1 {
		t.Fatalf("result = %+v", res)
	}
	if !exists(alive) {
		t.Error("an available simulator was removed")
	}

	// No full Xcode, or not macOS: nothing found.
	noXcode := &fakeRunner{installed: map[string]bool{"xcrun": true}, errs: map[string]error{"xcrun simctl list devices unavailable -j": errors.New("simctl not found")}}
	if rep := NewSimulators(testEnv(t.TempDir(), "darwin", nil, noXcode)).Scan(ctx, month); rep.Found {
		t.Errorf("without simctl: %+v", rep)
	}
	if rep := NewSimulators(testEnv(home, "linux", nil, r)).Scan(ctx, month); rep.Found {
		t.Error("simulators are macOS only")
	}
	if res := s.Clean(ctx, &Report{}, month); res.Removed != 0 {
		t.Error("no plan, nothing removed")
	}
	bad := &fakeRunner{installed: map[string]bool{"xcrun": true}, out: map[string]string{"xcrun simctl list devices unavailable -j": "not json"}}
	if _, ok := NewSimulators(testEnv(home, "darwin", nil, bad)).(*simulatorsSource).unavailable(ctx); ok {
		t.Error("bad JSON accepted")
	}
	if strings.Contains(string(out), "EVIL") && exists(filepath.Join(home, "elsewhere")) {
		t.Error("a path outside Devices must never be touched")
	}
}
