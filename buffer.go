package kcp

import (
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

var (
	stepSize          = []int32{64, 128, 256, 384, 512, 768, 1024, 1280, 1536}
	stepPools         []*sync.Pool
	stored            sync.Map
	userGetBufferFunc func(int32) []byte
	userPutBufferFunc func([]byte)
)

func init() {
	stepPools = make([]*sync.Pool, len(stepSize))
	for i := 0; i < len(stepSize); i++ {
		s := stepSize[i]
		stepPools[i] = &sync.Pool{
			New: func() any {
				buf := make([]byte, s)
				return buf
			},
		}
	}
}

func isOutBufferRange(s int32) bool {
	return s > stepSize[len(stepSize)-1]
}

func _getBuffer(s int32, store bool) []byte {
	for i := 0; i < len(stepSize); i++ {
		if stepSize[i] >= s {
			b := stepPools[i].Get().([]byte)
			if store {
				pb := (*reflect.SliceHeader)(unsafe.Pointer(&b)).Data
				stored.Store(pb, true)
			}
			return b
		}
	}
	return nil
}

func getBuffer(s int32) []byte {
	if userGetBufferFunc != nil {
		return userGetBufferFunc(s)
	}
	return _getBuffer(s, false)
}

func _putBuffer(b []byte, store bool) {
	c := cap(b)
	found := false
	for i := 0; i < len(stepSize); i++ {
		if c == int(stepSize[i]) {
			stepPools[i].Put(b)
			found = true
			break
		}
	}
	if !found {
		panic(fmt.Sprintf("kcpgo: not found buffer %v", b))
	} else if store {
		pb := (*reflect.SliceHeader)(unsafe.Pointer(&b)).Data
		if _, o := stored.LoadAndDelete(pb); !o {
			panic(fmt.Sprintf("kcpgo: not store buffer %v, cant put buffer", b))
		}
	}
}

func putBuffer(b []byte) {
	if userPutBufferFunc != nil {
		userPutBufferFunc(b)
		return
	}
	_putBuffer(b, false)
}

func allocOutputBuffer(s int32) []byte {
	var b []byte
	if userGetBufferFunc != nil {
		b = userGetBufferFunc(s)
	} else {
		b = _getBuffer(s, false)
	}
	return b[:s]
}

func UserMtuBufferFunc(get func(int32) []byte, put func([]byte)) {
	userGetBufferFunc = get
	userPutBufferFunc = put
}

func RecycleOutputBuffer(b []byte) {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	if sh.Len < sh.Cap {
		sh.Len = sh.Cap
	}
	if userPutBufferFunc != nil {
		userPutBufferFunc(b)
	} else {
		_putBuffer(b, false)
	}
}
