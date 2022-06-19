package kcp

import (
	"math/rand"
	"testing"
	"time"
)

func TestBuffer(t *testing.T) {
	var (
		maxSize int32 = 3000
		r             = rand.New(rand.NewSource(time.Now().UnixNano()))
		bufList [][]byte
		length  = 10000000
	)

	for i := 0; i < length; i++ {
		buf := getBuffer(r.Int31n(maxSize + 1))
		if buf != nil {
			bufList = append(bufList, buf)
		}
	}

	for i := 0; i < len(bufList); i++ {
		putBuffer(bufList[i])
	}
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
