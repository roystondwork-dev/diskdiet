package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/roystondwork-dev/diskdiet/internal/appcore"
	"github.com/roystondwork-dev/diskdiet/internal/buildinfo"
)

// DiskService is what the window can call. It forwards to appcore, which holds
// the logic and its tests, and turns progress into window events.
type DiskService struct {
	core *appcore.Core
	app  *application.App
}

func (s *DiskService) emit(p appcore.Progress) {
	if s.app != nil {
		s.app.Event.Emit("progress", p)
	}
}

// Scan measures every cache.
func (s *DiskService) Scan(ctx context.Context, req appcore.ScanRequest) (*appcore.Summary, error) {
	return s.core.Scan(ctx, req, s.emit) //nolint:wrapcheck // appcore errors are written for people
}

// Clean removes what the last scan planned for the chosen caches.
func (s *DiskService) Clean(ctx context.Context, req appcore.CleanRequest) (*appcore.CleanSummary, error) {
	return s.core.Clean(ctx, req, s.emit) //nolint:wrapcheck // appcore errors are written for people
}

// Reveal shows a folder from the last scan in the file manager.
func (s *DiskService) Reveal(path string) error {
	return s.core.Reveal(path) //nolint:wrapcheck // appcore errors are written for people
}

// Version is the app's version.
func (s *DiskService) Version() string {
	return buildinfo.Get().Version
}
