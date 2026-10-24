//go:build !(linux || darwin || freebsd || windows)

package admin

import "errors"

func diskUsage(path string) (key string, free, total uint64, err error) {
	return "", 0, 0, errors.New("no disponible en este sistema")
}
