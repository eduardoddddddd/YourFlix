//go:build windows

package admin

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// diskUsage: espacio libre (para el usuario) y total de la unidad que contiene path.
func diskUsage(path string) (key string, free, total uint64, err error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", 0, 0, err
	}
	var avail, tot, totFree uint64
	r, _, e := getDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&tot)), uintptr(unsafe.Pointer(&totFree)))
	if r == 0 {
		return "", 0, 0, e
	}
	key = strings.ToUpper(filepath.VolumeName(path))
	if key == "" {
		key = path
	}
	return key, avail, tot, nil
}
