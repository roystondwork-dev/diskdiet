//go:build !windows

package fsx

import (
	"io/fs"
	"syscall"
)

type fileStat struct {
	bytes int64
	links uint64
	id    fileID
}

// stat reports the space a file takes on disk (allocated blocks, as du counts
// them), its hard-link count and its identity.
func stat(_ string, info fs.FileInfo, _ bool) fileStat {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileStat{bytes: info.Size(), links: 1}
	}
	return fileStat{
		bytes: int64(st.Blocks) * 512,                           //nolint:unconvert // Blocks is int64 or uint64 depending on the OS
		links: uint64(st.Nlink),                                 //nolint:unconvert // Nlink width differs per OS
		id:    fileID{dev: uint64(st.Dev), ino: uint64(st.Ino)}, //nolint:unconvert // widths differ per OS
	}
}
