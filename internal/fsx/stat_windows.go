//go:build windows

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

// stat reports a file's size and, when countLinks is set, its hard-link count
// and identity. Windows only exposes link counts through an open handle, so
// they are read only when asked for.
func stat(path string, info fs.FileInfo, countLinks bool) fileStat {
	fs := fileStat{bytes: info.Size(), links: 1}
	if !countLinks || !info.Mode().IsRegular() {
		return fs
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return fs
	}
	h, err := syscall.CreateFile(p, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fs
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // read-only handle
	var d syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &d); err != nil {
		return fs
	}
	fs.links = uint64(d.NumberOfLinks)
	fs.id = fileID{dev: uint64(d.VolumeSerialNumber), ino: uint64(d.FileIndexHigh)<<32 | uint64(d.FileIndexLow)}
	return fs
}
