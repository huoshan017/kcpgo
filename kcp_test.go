package kcp

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/huoshan017/ponu/list"
)

var (
	initMilli = time.Now().UnixMilli()
)

func currentMilli() int32 {
	return int32(time.Now().UnixMilli() - initMilli)
}

type delayPacket struct {
	data []byte
	ts   int32
}

func newDelayPacket(data []byte) *delayPacket {
	var d = make([]byte, len(data))
	copy(d, data)
	return &delayPacket{data: d}
}

func (d delayPacket) getData() []byte {
	return d.data
}

func (d delayPacket) getTs() int32 {
	return d.ts
}

func (d *delayPacket) setTs(ts int32) {
	d.ts = ts
}

type random struct {
	size  int32
	seeds []int32
	ran   *rand.Rand
}

func newRandom(s int32, r *rand.Rand) *random {
	return &random{
		seeds: make([]int32, s),
		ran:   r,
	}
}

func (r *random) rand() int32 {
	if len(r.seeds) == 0 {
		return 0
	}
	if r.size == 0 {
		for i := int32(0); i < int32(len(r.seeds)); i++ {
			r.seeds[i] = i
		}
		r.size = int32(len(r.seeds))
	}
	var n = r.ran.Int31n(r.size)
	var x = r.seeds[n]
	r.size -= 1
	r.seeds[n] = r.seeds[r.size]
	return x
}

type latencySimulator struct {
	t              *testing.T
	tx             [2]int32
	current        int32
	lostrate       int32
	rttmin, rttmax int32
	nmax           int32
	p              [2]*list.List
	rand           [2]*random
	r              [2]*rand.Rand
}

func newLatencySimulator(t *testing.T, lostrate, rttmin, rttmax, nmax int32) *latencySimulator {
	s := &latencySimulator{}
	s.t = t
	s.current = currentMilli()
	s.lostrate = lostrate / 2
	s.rttmin = rttmin / 2
	s.rttmax = rttmax / 2
	s.nmax = nmax
	for i := 0; i < 2; i++ {
		s.tx[i] = 0
		s.r[i] = rand.New(rand.NewSource(time.Now().UnixNano() + int64(i)))
		s.rand[i] = newRandom(100, s.r[i])
		s.p[i] = list.New()
	}
	return s
}

func (s *latencySimulator) clear() {
	for i := 0; i < 2; i++ {
		s.p[i].Clear()
	}
}

func (s *latencySimulator) send(peer int32, data []byte) {
	s.tx[peer] += 1
	var r = s.rand[peer].rand()
	if r < s.lostrate {
		return
	}
	if s.p[peer].GetLength() >= s.nmax {
		return
	}
	var packet = newDelayPacket(data)
	var delay = s.rttmin
	if s.rttmax > s.rttmin {
		delay += s.r[peer].Int31n(s.rttmax - s.rttmin)
	}
	var current = currentMilli()
	packet.setTs(current + delay)
	s.p[peer].PushBack(packet)
}

func (s *latencySimulator) recv(peer int32, data []byte) int32 {
	peer = (peer + 1) % 2
	if s.p[peer].GetLength() == 0 {
		return -1
	}
	var iter = s.p[peer].Begin()
	var packet = iter.Value().(*delayPacket)
	if currentMilli() < packet.getTs() {
		return -2
	}
	if len(data) < len(packet.getData()) {
		return -3
	}
	s.p[peer].Delete(iter)
	var n = copy(data, packet.getData())
	return int32(n)
}

var (
	vnet *latencySimulator
)

func output(data []byte, user any) int32 {
	var peer = user.(int32)
	vnet.send(peer, data)
	return 0
}

func test(mode int32, t *testing.T) {
	vnet = newLatencySimulator(t, 10, 60, 125, 1000)

	var (
		kcps                                     = [2]*KcpCB{}
		current                            int32 = currentMilli()
		slap                               int32 = current + 20
		index, next, sumrtt, count, maxrtt int32
	)

	switch mode {
	case 0:
		kcps[0] = New(0x11223344, int32(0), output, WithWnd(128, 128), WithInterval(10))
		kcps[1] = New(0x11223344, int32(1), output, WithWnd(128, 128), WithInterval(10))
	case 1:
		kcps[0] = New(0x11223344, int32(0), output, WithWnd(128, 128), WithInterval(10), WithNoCwnd(true))
		kcps[1] = New(0x11223344, int32(1), output, WithWnd(128, 128), WithInterval(10), WithNoCwnd(true))
	default:
		kcps[0] = New(0x11223344, int32(0), output, WithWnd(128, 128), WithNodelay(2), WithInterval(10), WithFastResend(2), WithNoCwnd(true))
		kcps[1] = New(0x11223344, int32(1), output, WithWnd(128, 128), WithNodelay(2), WithInterval(10), WithFastResend(1), WithNoCwnd(true), WithMinRTO(10))
	}

	var buffer [1500]byte
	var start = currentMilli()
	for {
		time.Sleep(time.Millisecond)
		current = currentMilli()
		for i := 0; i < 2; i++ {
			kcps[i].Update(current)
		}

		for ; current >= slap; slap += 20 {
			encode32(buffer[:], index)
			index += 1
			encode32(buffer[4:], current)
			var data = buffer[:8]
			kcps[0].Send(data)
		}

		for {
			var d = vnet.recv(1, buffer[:])
			if d < 0 {
				break
			}
			kcps[1].Input(buffer[:d])
		}

		for {
			var d = vnet.recv(0, buffer[:])
			if d < 0 {
				break
			}
			kcps[0].Input(buffer[:d])
		}

		for {
			var d = kcps[1].Recv(buffer[:])
			if d < 0 {
				break
			}
			kcps[1].Send(buffer[:d])
		}

		for {
			var d = kcps[0].Recv(buffer[:])
			if d < 0 {
				break
			}
			var sn = decode32(buffer[:])
			var ts = decode32(buffer[4:])
			var rtt = current - ts
			if sn != next {
				t.Errorf("ERROR sn %v  next %v  count %v", sn, next, count)
				return
			}

			next += 1
			sumrtt += rtt
			count += 1
			if rtt > maxrtt {
				maxrtt = rtt
			}

			t.Logf("[RECV] mode=%v sn=%v rtt=%v, next=%v", mode, sn, rtt, next)
		}

		if next > 1000 {
			break
		}
	}

	vnet.clear()

	var cost = currentMilli() - start

	for i := 0; i < 2; i++ {
		kcps[i].Release()
	}

	var names = []string{
		"default", "normal", "fast",
	}
	t.Logf("%v mode result (%dms):", names[mode], cost)
	t.Logf("avgrtt=%v maxrtt=%v tx=%v", sumrtt/count, maxrtt, vnet.tx[0])
}

func TestDefaultKCP(t *testing.T) {
	test(0, t)
}

func TestNormalKCP(t *testing.T) {
	test(1, t)
}

func TestFastKCP(t *testing.T) {
	test(2, t)
}

var letters = []byte("abcdefghijklmnopqrstuvwxyz01234567890~!@#$%^&*()_+-={}[]|:;'<>?/.,")
var lettersLen = len(letters)

func randBytes(n int, ran *rand.Rand) []byte {
	b := make([]byte, n+2)
	encode16(b, int16(n))
	for i := 2; i < len(b); i++ {
		r := ran.Int31n(int32(lettersLen))
		b[i] = letters[r]
	}
	return b
}

func testStreamKCP(t *testing.T, nocwnd bool) {
	vnet = newLatencySimulator(t, 30, 60, 125, 1000)

	var (
		kcps = [2]*KcpCB{
			New(0x11223344, int32(0), output, WithStream(true), WithWnd(128, 128), WithInterval(10), WithNoCwnd(nocwnd)),
			New(0x11223344, int32(1), output, WithStream(true), WithWnd(128, 128), WithInterval(10), WithNoCwnd(nocwnd)),
		}
		current    int32 = currentMilli()
		slap       int32 = current + 20
		next       int32
		dataList   [][]byte
		maxDataLen int32      = 4500
		ran        *rand.Rand = rand.New(rand.NewSource(time.Now().Unix()))
	)

	var buffer [5000]byte
	var resultBuf bytes.Buffer
	var dlen int16
	var start = currentMilli()
	for {
		time.Sleep(time.Millisecond)
		current = currentMilli()
		for i := 0; i < 2; i++ {
			kcps[i].Update(current)
		}

		for ; current >= slap; slap += 20 {
			var l = ran.Int31n(maxDataLen) + 1
			var data = randBytes(int(l), ran)
			dataList = append(dataList, data[2:])
			kcps[0].Send(data)
		}

		for {
			var d = vnet.recv(1, buffer[:])
			if d < 0 {
				break
			}
			kcps[1].Input(buffer[:d])
		}

		for {
			var d = vnet.recv(0, buffer[:])
			if d < 0 {
				break
			}
			kcps[0].Input(buffer[:d])
		}

		for {
			var d = kcps[1].Recv(buffer[:])
			if d < 0 {
				break
			}
			kcps[1].Send(buffer[:d])
		}

		var n int32
		for {
			var d = kcps[0].Recv(buffer[:])
			if d < 0 {
				break
			}
			resultBuf.Write(buffer[:d])
			n += d
		}

		if n > 0 {
			for {
				if resultBuf.Len() < 2 {
					break
				}
				if dlen == 0 {
					resultBuf.Read(buffer[:2])
					dlen = decode16(buffer[:2])
				}
				if resultBuf.Len() < int(dlen) {
					break
				}
				resultBuf.Read(buffer[:dlen])
				if !bytes.Equal(buffer[:dlen], dataList[0]) {
					panic(fmt.Sprintf("received index %v data compare failed\r\nbuffer[:dl] %v\r\ndataList[next] %v", next, buffer[:dlen], dataList[0]))
				}
				dataList = dataList[1:]
				if dlen < 100 {
					t.Logf("[RECV] sn=%v, data=%v", next, buffer[:dlen])
				} else {
					t.Logf("[RECV] sn=%v", next)
				}
				next += 1
				dlen = 0
			}
		}
		if next > 1000 {
			break
		}
	}
	var cost = currentMilli() - start
	vnet.clear()
	for i := 0; i < 2; i++ {
		kcps[i].Release()
	}
	t.Logf("stream mode result (%dms):", cost)
}

func TestStreamWithCwnd(t *testing.T) {
	testStreamKCP(t, false)
}

func TestStreamNoCwnd(t *testing.T) {
	testStreamKCP(t, true)
}
