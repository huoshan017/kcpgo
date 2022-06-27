package kcp

import "sync"

var (
	stepSize  = []int32{64, 128, 256, 384, 512, 768, 1024, 1280, 1536}
	stepPools []*sync.Pool
)

func init() {
	stepPools = make([]*sync.Pool, len(stepSize))
	for i := 0; i < len(stepSize); i++ {
		s := stepSize[i]
		stepPools[i] = &sync.Pool{
			New: func() any {
				return make([]byte, s)
			},
		}
	}
}

func isOutBufferRange(s int32) bool {
	return s > stepSize[len(stepSize)-1]
}

func getBuffer(s int32) []byte {
	for i := 0; i < len(stepSize); i++ {
		if stepSize[i] >= s {
			b := stepPools[i].Get().([]byte)
			return b
		}
	}
	return nil
}

func putBuffer(b []byte) {
	c := cap(b)
	for i := 0; i < len(stepSize); i++ {
		if c == int(stepSize[i]) {
			stepPools[i].Put(b)
		}
	}
}
