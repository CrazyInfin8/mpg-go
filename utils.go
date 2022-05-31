package mpg

import (
	"time"
	"unsafe"

	"github.com/gotranspile/cxgo/runtime/libc"
)

func index(data *uint8, i int) byte {
	ptr := unsafe.Add(unsafe.Pointer(data), unsafe.Sizeof(uint8(0))*uintptr(i))
	return *(*byte)(ptr)
}

func bytesToUintPtr(data []byte) *uint8 {
	return (*uint8)(unsafe.Pointer(&data[0]))
}

func boolToInt(t bool) int64 {
	if t {
		return _true
	} else {
		return _false
	}
}

func floatToSecs(t float64) time.Duration {
	return time.Duration(t * float64(time.Second))
}

// SetAlpha is a helper function intended to set the alpha channel of
// "*image.RGBA.Pix" byte array since functions like "ReadRGBA" and "ReadRGBAAt"
// does not set alpha.
func SetAlpha(a byte, data []byte) {
	for i := 3; i < len(data); i += 4 {
		data[i] = a
	}
}

// Avoids: https://github.com/gotranspile/cxgo/issues/55
func realloc(buf unsafe.Pointer, oldSize, newSize int) unsafe.Pointer {
	p := libc.Malloc(newSize)
	bdst := unsafe.Slice((*byte)(p), newSize)
	bsrc := unsafe.Slice((*byte)(buf), oldSize)
	copy(bdst, bsrc)
	return p
}
