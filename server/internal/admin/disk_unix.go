//go:build linux || darwin || freebsd

package admin

import (
	"fmt"
	"os"
	"syscall"
)

// diskUsage: espacio libre (para el usuario) y total del disco que contiene path; key
// identifica el disco para no repetirlo si dos fuentes están en el mismo.
func diskUsage(path string) (key string, free, total uint64, err error) {
	var st syscall.Statfs_t
	if err = syscall.Statfs(path, &st); err != nil {
		return "", 0, 0, err
	}
	free = uint64(st.Bavail) * uint64(st.Bsize)
	total = uint64(st.Blocks) * uint64(st.Bsize)
	key = path
	if fi, e := os.Stat(path); e == nil {
		if s, ok := fi.Sys().(*syscall.Stat_t); ok {
			key = fmt.Sprint("dev:", s.Dev)
		}
	}
	return key, free, total, nil
}
