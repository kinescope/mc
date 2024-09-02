//go:build !appengine
// +build !appengine

package mc

import (
	"unsafe"
)

type sliceHeader struct {
	s   string
	cap int
}

func str2byte(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&sliceHeader{s, len(s)}))
}
