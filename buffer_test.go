package kcp

import (
	"math/rand"
	"testing"
	"time"
	"unsafe"
)

func testBuffer(minSize, maxSize int32) {
	var (
		r       = rand.New(rand.NewSource(time.Now().UnixNano()))
		bufList [][]byte
		length  = 10000000
	)

	for i := 0; i < length; i++ {
		buf := getBuffer(r.Int31n(maxSize) + minSize)
		if buf != nil {
			bufList = append(bufList, buf)
		}
	}

	for i := 0; i < len(bufList); i++ {
		putBuffer(bufList[i])
	}
}

func TestBuffer(t *testing.T) {
	testBuffer(1, 1500)
}

func BenchmarkBuffer(t *testing.B) {
	var (
		maxSize int32 = 3000
		r             = rand.New(rand.NewSource(time.Now().UnixNano()))
		bufList [][]byte
	)
	for i := 0; i < t.N; i++ {
		buf := getBuffer(r.Int31n(maxSize + 1))
		if buf != nil {
			bufList = append(bufList, buf)
		}
	}
	for i := 0; i < len(bufList); i++ {
		putBuffer(bufList[i])
	}
}

func TestMtuBufferFunc(t *testing.T) {
	UserMtuBufferFunc(func(s int32) []byte {
		s = 4 + s + 4
		b := _getBuffer(s, true)
		// 截取中間一段，去掉前後綴8個字節
		return b[4 : s-4]
	}, func(b []byte) {
		//sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
		//if sh.Len < sh.Cap {
		//	sh.Len = sh.Cap
		//}
		//sh.Data -= 4
		sd := unsafe.SliceData(b)
		b = unsafe.Slice((*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(sd))-4)), cap(b)+4)
		_putBuffer(b, true)
	})
	testBuffer(1, 1500-4-4)
}
