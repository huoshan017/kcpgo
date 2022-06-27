package kcp

import (
	"fmt"
	"sync"

	"github.com/huoshan017/ponu/list"
)

type segment struct {
	conv     uint32
	cmd      int32
	frg      int32
	wnd      int32
	ts       int32
	sn       int32
	una      int32
	resendts int32
	rto      int32
	fastack  int32
	xmit     int32
	data     []byte
	dlen     int32
}

func (s *segment) copyData(data []byte, mss int32) int32 {
	if isOutBufferRange(mss) {
		return -1
	}
	var (
		dlen = int32(len(data))
		nbuf []byte
	)
	if s.data == nil {
		if dlen < mss {
			nbuf = getBuffer(dlen)
		} else {
			nbuf = getBuffer(mss)
		}
		if int(mss) < len(nbuf) {
			nbuf = nbuf[:mss]
		}
		s.data = nbuf
	} else {
		var left = int32(len(s.data)) - s.dlen
		var canGrow = mss > int32(len(s.data))
		if left <= 0 && !canGrow {
			return 0
		}
		// need grow
		if dlen > left && canGrow { // left space of s.data is not enough
			// s.data length is max
			if dlen+s.dlen < mss {
				nbuf = getBuffer(dlen + s.dlen)
			} else {
				nbuf = getBuffer(mss)
			}
			if int(mss) < len(nbuf) {
				nbuf = nbuf[:mss]
			}
			copy(nbuf, s.data[:s.dlen])
			putBuffer(s.data)
			s.data = nbuf
		}
	}
	var copied = int32(copy(s.data[s.dlen:], data))
	s.dlen += copied
	return copied
}

func (s *segment) clear() {
	s.conv = 0
	s.cmd = 0
	s.frg = 0
	s.wnd = 0
	s.ts = 0
	s.sn = 0
	s.una = 0
	s.resendts = 0
	s.rto = 0
	s.fastack = 0
	s.xmit = 0
	putBuffer(s.data)
	s.data = nil
	s.dlen = 0
}

type KcpCB struct {
	options                   Options
	conv                      uint32
	mss, state                int32
	snd_una, snd_nxt, rcv_nxt int32
	rx_rttval, rx_srtt        int32
	rmt_wnd, cwnd, probe      int32
	current, ts_flush         int32
	ts_probe, probe_wait      int32
	incr                      int32
	snd_queue                 list.List
	rcv_queue                 list.List
	snd_buf                   list.List
	rcv_buf                   list.List
	acklist                   []int32
	ackcount                  int32
	ackblock                  int32
	user                      interface{}
	buffer                    []byte
	output_func               func(buf []byte, len int32, user any) int32
	updated                   bool
}

// 创建kcp
func NewKcp(conv uint32, user any, sendFunc func([]byte, int32, any) int32, options ...Option) *KcpCB {
	kcp := &KcpCB{}
	for i := 0; i < len(options); i++ {
		options[i](&kcp.options)
	}
	kcp.conv = conv
	kcp.user = user
	kcp.snd_queue = list.NewObj()
	kcp.rcv_queue = list.NewObj()
	kcp.snd_buf = list.NewObj()
	kcp.rcv_buf = list.NewObj()
	kcp.output_func = sendFunc
	kcp.init()
	return kcp
}

func (k *KcpCB) init() {
	if k.options.GetSendWnd() == 0 {
		k.options.snd_wnd = KCP_WND_SND
	}
	if k.options.rcv_wnd == 0 {
		k.options.rcv_wnd = KCP_WND_RCV
	}
	k.rmt_wnd = k.options.rcv_wnd

	if k.options.mtu == 0 {
		k.options.mtu = KCP_MTU_DEF
	} else {
		k.SetMtu(k.options.mtu)
	}
	k.mss = k.options.mtu - KCP_OVERHEAD
	k.buffer = make([]byte, 3*(k.options.mtu+KCP_OVERHEAD))
	if k.options.rx_rto == 0 {
		k.options.rx_rto = KCP_RTO_DEF
	}
	if k.options.rx_minrto == 0 {
		k.options.rx_minrto = KCP_RTO_MIN
	}
	if k.options.nodelay > 0 {
		k.options.rx_minrto = KCP_RTO_NDL
	} else {
		k.options.rx_minrto = KCP_RTO_MIN
	}
	if k.options.interval == 0 {
		k.options.interval = KCP_INTERVAL
	} else {
		k.SetInterval(k.options.interval)
	}
	k.ts_flush = KCP_INTERVAL
	if k.options.ssthresh == 0 {
		k.options.ssthresh = KCP_THRESH_INIT
	}
	if k.options.fastlimit == 0 {
		k.options.fastlimit = KCP_FASTACK_LIMIT
	}
	if k.options.dead_link == 0 {
		k.options.dead_link = KCP_DEADLINK
	}
}

func (k *KcpCB) Release() {
	var (
		seg  *segment
		iter list.Iterator
	)
	if !k.snd_buf.IsEmpty() {
		iter = k.snd_buf.Begin()
		for iter != k.snd_buf.End() {
			seg = iter.Value().(*segment)
			putSeg(seg)
			iter = iter.Next()
		}
		k.snd_buf.Clear()
	}
	if !k.rcv_buf.IsEmpty() {
		iter = k.rcv_buf.Begin()
		for iter != k.rcv_buf.End() {
			seg = iter.Value().(*segment)
			putSeg(seg)
			iter = iter.Next()
		}
		k.rcv_buf.Clear()
	}
	if !k.snd_queue.IsEmpty() {
		iter = k.snd_queue.Begin()
		for iter != k.snd_queue.End() {
			seg = iter.Value().(*segment)
			putSeg(seg)
			iter = iter.Next()
		}
		k.snd_queue.Clear()
	}
	if !k.rcv_queue.IsEmpty() {
		iter = k.rcv_queue.Begin()
		for iter != k.rcv_queue.End() {
			seg = iter.Value().(*segment)
			putSeg(seg)
			iter = iter.Next()
		}
		k.rcv_queue.Clear()
	}
}

func (k *KcpCB) Recv(buf []byte) int32 {
	return k.recv(buf, false)
}

func (k *KcpCB) Peek(buf []byte) int32 {
	return k.recv(buf, true)
}

func (k *KcpCB) recv(buf []byte, isPeek bool) int32 {
	if k.rcv_queue.IsEmpty() {
		return -1
	}
	peekSize := k.peekSize()
	if peekSize < 0 {
		return -2
	}

	if peekSize > int32(len(buf)) {
		return -3
	}

	var recover bool
	if k.rcv_queue.GetLength() >= int32(k.options.rcv_wnd) {
		recover = true
	}

	var (
		length int32
		o      bool
		iter   = k.rcv_queue.Begin()
	)

	// merge fragment
	for iter != k.rcv_queue.End() {
		seg := iter.Value().(*segment)
		if buf != nil {
			copy(buf[length:], seg.data[:seg.dlen])
			length += seg.dlen
		}

		frg := seg.frg
		if !isPeek {
			iter, o = k.rcv_queue.DeleteContinueNext(iter)
			if !o {
				panic("kcp: cant delete node on merge fragment")
			}
			putSeg(seg)
		} else {
			iter = iter.Next()
		}

		if frg == 0 {
			break
		}
	}

	if length != peekSize {
		panic(fmt.Sprintf("kcp: peek size %v must equal to length %v with merge fragment", peekSize, length))
	}

	// move available data from rcv_buf -> rcv_queue
	iter = k.rcv_buf.Begin()
	for iter != k.rcv_buf.End() {
		seg := iter.Value().(*segment)
		if seg.sn != k.rcv_nxt || k.rcv_queue.GetLength() >= int32(k.options.rcv_wnd) {
			break
		}

		iter, o = k.rcv_buf.DeleteContinueNext(iter)
		if !o {
			panic("kcp: delete receive buf node failed")
		}
		k.rcv_queue.PushBack(iter.Value())
		k.rcv_nxt += 1
	}

	if recover && k.rcv_queue.GetLength() < int32(k.options.rcv_wnd) {
		k.probe |= KCP_ASK_TELL
	}

	return length
}

func (k *KcpCB) peekSize() int32 {
	if k.rcv_queue.IsEmpty() {
		return -1
	}
	iter := k.rcv_queue.Begin()
	seg := iter.Value().(*segment)
	if seg.frg == 0 {
		return seg.dlen
	}

	if k.rcv_queue.GetLength() < int32(seg.frg)+1 {
		return -1
	}

	var length int32
	for iter != k.rcv_queue.End() {
		seg = iter.Value().(*segment)
		length += seg.dlen
		if seg.frg == 0 {
			break
		}
	}

	return length
}

func (k *KcpCB) Send(data []byte) int32 {
	if len(data) == 0 {
		return -1
	}

	var copied int32
	// stream mode, append to previous segment in streaming mode (if possible)
	if k.options.stream {
		if !k.snd_queue.IsEmpty() {
			iter := k.snd_queue.RBegin()
			seg := iter.Value().(*segment)
			copied = seg.copyData(data, int32(k.mss))
			if copied < 0 {
				return -2
			}
			if copied > 0 {
				seg.frg = 0
			}
		}
		if copied >= int32(len(data)) {
			return 0
		}
	}

	var count int32
	left := int32(len(data)) - copied
	if left < int32(k.mss) {
		count = 1
	} else {
		count = (left + int32(k.mss) - 1) / int32(k.mss)
	}

	if count >= KCP_WND_RCV {
		return -2
	}

	if count == 0 {
		count = 1
	}

	// fragment
	for i := int32(0); i < count; i++ {
		var seg = getSeg()
		var c = seg.copyData(data[copied:], int32(k.mss))
		if c < 0 {
			putSeg(seg)
			return -2
		}
		if !k.options.stream {
			seg.frg = count - i - 1
		} else {
			seg.frg = 0
		}
		k.snd_queue.PushBack(seg)
		copied += c
	}

	return 0
}

func (k *KcpCB) IsDead() bool {
	return k.state == 0x7fffffff
}

func (k *KcpCB) updateAck(rtt int32) {
	var rto int32
	if k.rx_srtt == 0 {
		k.rx_srtt = rtt
		k.rx_rttval = rtt / 2
	} else {
		var delta = rtt - k.rx_srtt
		if delta < 0 {
			delta = -delta
		}
		k.rx_rttval = (3*k.rx_rttval + delta) / 4
		k.rx_srtt = (7*k.rx_srtt + rtt) / 8
		if k.rx_srtt < 1 {
			k.rx_srtt = 1
		}
	}
	rto = k.rx_srtt + max(k.options.interval, 4*k.rx_rttval)
	k.options.rx_rto = bound(k.options.rx_minrto, rto, KCP_RTO_MAX)
}

func (k *KcpCB) shrinkBuf() {
	var iter = k.snd_buf.Begin()
	if iter != k.snd_buf.End() {
		var seg = iter.Value().(*segment)
		k.snd_una = seg.sn
	} else {
		k.snd_una = k.snd_nxt
	}
}

func (k *KcpCB) parseAck(sn int32) {
	if timeDiff(sn, int32(k.snd_una)) < 0 || timeDiff(sn, int32(k.snd_nxt)) >= 0 {
		return
	}

	var iter = k.snd_buf.Begin()
	for iter != k.snd_buf.End() {
		var seg = iter.Value().(*segment)
		if int32(seg.sn) == sn {
			if !k.snd_buf.Delete(iter) {
				panic("kcp: parse ack delete node failed")
			}
			putSeg(seg)
			break
		}
		if timeDiff(sn, int32(seg.sn)) < 0 {
			break
		}
		iter = iter.Next()
	}
}

func (k *KcpCB) parseUna(una int32) {
	var iter = k.snd_buf.Begin()
	for iter != k.snd_buf.End() {
		var seg = iter.Value().(*segment)
		if timeDiff(una, int32(seg.sn)) <= 0 {
			break
		}
		var o bool
		iter, o = k.snd_buf.DeleteContinueNext(iter)
		if !o {
			panic("kcp: parse una delete node failed")
		}
		putSeg(seg)
	}
}

func (k *KcpCB) parseFastAck(sn, ts int32) {
	if timeDiff(sn, k.snd_una) < 0 || timeDiff(sn, k.snd_nxt) >= 0 {
		return
	}
	var iter = k.snd_buf.Begin()
	for iter != k.snd_buf.End() {
		var seg = iter.Value().(*segment)
		if timeDiff(sn, seg.sn) < 0 {
			break
		}
		if sn != seg.sn {
			if !KCP_FASTACK_CONSERVE {
				seg.fastack += 1
			} else {
				if timeDiff(ts, seg.ts) >= 0 {
					seg.fastack += 1
				}
			}
		}
		iter = iter.Next()
	}
}

func (k *KcpCB) ackPush(sn, ts int32) {
	var newSize = k.ackcount + 1
	if newSize > k.ackblock {
		var newBlock int32
		for newBlock = 8; newBlock < newSize; newBlock <<= 1 {
		}
		var ackList = make([]int32, newBlock*2)
		if k.acklist != nil {
			for i := int32(0); i < k.ackcount; i++ {
				ackList[i*2+0] = k.acklist[i*2+0]
				ackList[i*2+1] = k.acklist[i*2+1]
			}
		}
		k.acklist = ackList
		k.ackblock = newBlock
	}
	k.acklist[k.ackcount*2] = sn
	k.acklist[k.ackcount*2+1] = ts
	k.ackcount += 1
}

func (k *KcpCB) ackGet(n int32) (int32, int32) {
	return k.acklist[n*2], k.acklist[n*2+1]
}

func (k *KcpCB) parseData(seg *segment) {
	var (
		sn     = seg.sn
		repeat bool
	)
	if timeDiff(sn, k.rcv_nxt+k.options.rcv_wnd) >= 0 || timeDiff(sn, k.rcv_nxt) < 0 {
		putSeg(seg)
		return
	}

	var iter = k.rcv_buf.RBegin()
	for iter != k.rcv_buf.REnd() {
		var tseg = iter.Value().(*segment)
		if tseg.sn == sn {
			repeat = true
			break
		}
		if timeDiff(sn, tseg.sn) > 0 {
			break
		}
		iter = iter.Prev()
	}

	if !repeat {
		k.rcv_buf.Insert(seg, iter)
	} else {
		putSeg(seg)
	}

	// move available data from rcv_buf -> rcv_queue
	iter = k.rcv_buf.Begin()
	for iter != k.rcv_buf.End() {
		seg = iter.Value().(*segment)
		if seg.sn == k.rcv_nxt && k.rcv_queue.GetLength() < k.options.rcv_wnd {
			var o bool
			iter, o = k.rcv_buf.DeleteContinueNext(iter)
			if !o {
				panic("kcp: parse data failed during delete node from receive queue")
			}
			k.rcv_queue.PushBack(seg)
			k.rcv_nxt += 1
		} else {
			break
		}
	}
}

func (k *KcpCB) Input(data []byte) int32 {
	var prevUna = k.snd_una
	if data == nil || len(data) < KCP_OVERHEAD {
		return -1
	}

	var (
		flag             bool
		maxAck, latestTs int32
		offset           int32
		size             = int32(len(data))
	)
	for {
		if size < KCP_OVERHEAD {
			break
		}

		var conv = uint32(decode32(data[offset:]))
		offset += 4
		if conv != k.conv {
			return -1
		}
		var cmd = int32(data[offset])
		offset += 1
		var frg = int32(data[offset])
		offset += 1
		var wnd = decode16(data[offset:])
		offset += 2
		var ts = decode32(data[offset:])
		offset += 4
		var sn = decode32(data[offset:])
		offset += 4
		var una = decode32(data[offset:])
		offset += 4
		var dlen = decode32(data[offset:])
		offset += 4

		size -= KCP_OVERHEAD

		if size < dlen || dlen < 0 {
			return -2
		}

		if cmd != KCP_CMD_PUSH && cmd != KCP_CMD_ACK && cmd != KCP_CMD_WASK && cmd != KCP_CMD_WINS {
			return -3
		}

		k.rmt_wnd = int32(wnd)
		k.parseUna(una)
		k.shrinkBuf()

		switch cmd {
		case KCP_CMD_ACK:
			if timeDiff(k.current, ts) >= 0 {
				k.updateAck(timeDiff(k.current, ts))
			}
			k.parseAck(sn)
			k.shrinkBuf()
			if !flag {
				flag = true
				maxAck = sn
				latestTs = ts
			} else {
				if timeDiff(sn, maxAck) > 0 {
					if KCP_FASTACK_CONSERVE {
						maxAck = sn
						latestTs = ts
					} else {
						if timeDiff(ts, latestTs) > 0 {
							maxAck = sn
							latestTs = ts
						}
					}
				}
			}
		case KCP_CMD_PUSH:
			if timeDiff(sn, k.rcv_nxt+k.options.rcv_wnd) < 0 {
				k.ackPush(sn, ts)
				if timeDiff(sn, k.rcv_nxt) >= 0 {
					var seg = getSeg()
					seg.conv = conv
					seg.cmd = cmd
					seg.frg = frg
					seg.wnd = int32(wnd)
					seg.ts = ts
					seg.sn = sn
					seg.una = una
					if dlen > 0 {
						seg.copyData(data[offset:offset+dlen], int32(k.mss))
					}
					k.parseData(seg)
				}
			}
		case KCP_CMD_WASK:
			k.probe |= KCP_ASK_TELL
		case KCP_CMD_WINS:
			// do nothing
		default:
			return -3
		}

		offset += dlen
		size -= dlen
	}

	if flag {
		k.parseFastAck(maxAck, latestTs)
	}

	if timeDiff(k.snd_una, prevUna) > 0 {
		if k.cwnd < k.rmt_wnd {
			var mss int32 = k.mss
			if k.cwnd < k.options.ssthresh {
				k.cwnd += 1
				k.incr += mss
			} else {
				if k.incr < mss {
					k.incr = mss
				}
				k.incr += (mss*mss)/k.incr + (mss / 16)
				if (k.cwnd+1)*mss <= k.incr {
					k.cwnd = (k.incr + mss - 1) / (mss)
				}
			}
			if k.cwnd > k.rmt_wnd {
				k.cwnd = k.rmt_wnd
				k.incr = k.rmt_wnd * mss
			}
		}
	}

	return 0
}

func (k *KcpCB) encodeSeg(data []byte, seg *segment) int32 {
	var offset int32
	encode32(data, int32(seg.conv))
	offset += 4
	data[offset] = byte(seg.cmd)
	offset += 1
	data[offset] = byte(seg.frg)
	offset += 1
	encode16(data[offset:], int16(seg.wnd))
	offset += 2
	encode32(data[offset:], seg.ts)
	offset += 4
	encode32(data[offset:], seg.sn)
	offset += 4
	encode32(data[offset:], seg.una)
	offset += 4
	encode32(data[offset:], seg.dlen)
	offset += 4
	return offset
}

func (k *KcpCB) wndUnused() int32 {
	if k.rcv_queue.GetLength() < k.options.rcv_wnd {
		return k.options.rcv_wnd - k.rcv_queue.GetLength()
	}
	return 0
}

func (k *KcpCB) output(data []byte, dlen int32) {
	k.output_func(data, dlen, k.user)
}

func (k *KcpCB) Flush() {
	if !k.updated {
		return
	}

	var seg segment
	seg.conv = k.conv
	seg.frg = 0
	seg.wnd = k.wndUnused()
	seg.una = k.rcv_nxt
	seg.dlen = 0
	seg.sn = 0
	seg.ts = 0

	// flush acknowledges
	var (
		count  = k.ackcount
		offset int32
	)
	for i := int32(0); i < count; i++ {
		if offset+KCP_OVERHEAD > k.options.mtu {
			k.output(k.buffer, offset)
			offset = 0
		}
		seg.cmd = KCP_CMD_ACK
		seg.sn, seg.ts = k.ackGet(i)
		var d = k.encodeSeg(k.buffer[offset:], &seg)
		offset += d
	}

	k.ackcount = 0

	// probe window size (if remote window size equals zero)
	if k.rmt_wnd == 0 {
		if k.probe_wait == 0 {
			k.probe_wait = KCP_PROBE_INIT
			k.ts_probe = k.current + k.probe_wait
		} else {
			if timeDiff(k.current, k.ts_probe) >= 0 {
				if k.probe_wait < KCP_PROBE_INIT {
					k.probe_wait = KCP_PROBE_INIT
				}
				k.probe_wait += k.probe_wait / 2
				if k.probe_wait > KCP_PROBE_LIMIT {
					k.probe_wait = KCP_PROBE_LIMIT
				}
				k.ts_probe = k.current + k.probe_wait
				k.probe |= KCP_ASK_SEND
			}
		}
	} else {
		k.ts_probe = 0
		k.probe_wait = 0
	}

	// flush window probing commands
	if k.probe&KCP_ASK_SEND > 0 {
		seg.cmd = KCP_CMD_WASK
		if offset+KCP_OVERHEAD > k.options.mtu {
			k.output(k.buffer, offset)
			offset = 0
		}
		var d = k.encodeSeg(k.buffer[offset:], &seg)
		offset += d
	}

	// flush window probing commands
	if k.probe&KCP_ASK_TELL > 0 {
		seg.cmd = KCP_CMD_WINS
		if offset+KCP_OVERHEAD > k.options.mtu {
			k.output(k.buffer, offset)
			offset = 0
		}
		var d = k.encodeSeg(k.buffer[offset:], &seg)
		offset += d
	}

	k.probe = 0

	// calculate window size
	var cwnd = min(k.options.snd_wnd, k.rmt_wnd)
	if !k.options.nocwnd {
		cwnd = min(k.cwnd, cwnd)
	}

	// move data from snd_queue to snd_buf
	for timeDiff(k.snd_nxt, k.snd_una+cwnd) < 0 {
		if k.snd_queue.IsEmpty() {
			break
		}
		var iter = k.snd_queue.Begin()
		var tseg = iter.Value().(*segment)
		o := k.snd_queue.Delete(iter)
		if !o {
			panic("kcp: flush delete node failed")
		}
		k.snd_buf.PushBack(tseg)
		tseg.conv = k.conv
		tseg.cmd = KCP_CMD_PUSH
		tseg.wnd = seg.wnd
		tseg.ts = k.current
		tseg.sn = k.snd_nxt
		k.snd_nxt += 1
		tseg.una = k.rcv_nxt
		tseg.resendts = k.current
		tseg.rto = k.options.rx_rto
		tseg.fastack = 0
		tseg.xmit = 0
	}

	// calculate resent
	var resent = func() int32 {
		if k.options.fastresend > 0 {
			return k.options.fastresend
		}
		return 0x7fffffff
	}()
	var rtomin = func() int32 {
		if k.options.nodelay == 0 {
			return k.options.rx_rto >> 3
		}
		return 0
	}()

	var (
		change int32
		lost   bool
	)
	// flush data segments
	for iter := k.snd_buf.Begin(); iter != k.snd_buf.End(); iter = iter.Next() {
		var tseg = iter.Value().(*segment)
		var needSend bool
		if tseg.xmit == 0 {
			tseg.xmit += 1
			tseg.rto = k.options.rx_rto
			tseg.resendts = k.current + tseg.rto + rtomin
			needSend = true
		} else if diff := timeDiff(k.current, tseg.resendts); diff >= 0 {
			tseg.xmit += 1
			if k.options.nodelay == 0 {
				tseg.rto += max(tseg.rto, k.options.rx_rto)
			} else {
				var step = func() int32 {
					if k.options.nodelay < 2 {
						return tseg.rto
					}
					return k.options.rx_rto
				}()
				tseg.rto += step / 2
			}
			tseg.resendts = k.current + tseg.rto
			lost = true
			needSend = true
		} else if tseg.fastack >= resent {
			if tseg.xmit <= k.options.fastlimit || k.options.fastlimit <= 0 {
				tseg.xmit += 1
				tseg.fastack = 0
				tseg.resendts = k.current + tseg.rto
				change += 1
				needSend = true
			}
		}
		if needSend {
			tseg.ts = k.current
			tseg.wnd = seg.wnd
			tseg.una = k.rcv_nxt
			var need = KCP_OVERHEAD + tseg.dlen
			if offset+need > k.options.mtu {
				k.output(k.buffer, offset)
				offset = 0
			}
			var d = k.encodeSeg(k.buffer[offset:], tseg)
			offset += d

			if tseg.dlen > 0 {
				copy(k.buffer[offset:], tseg.data[:tseg.dlen])
				offset += tseg.dlen
			}

			if tseg.xmit >= k.options.dead_link {
				k.state = 0x7fffffff
			}
		}
	}

	// flush remain segments
	if offset > 0 {
		k.output(k.buffer, offset)
	}

	// update ssthresh
	if change > 0 {
		var inflight = k.snd_nxt - k.snd_una
		k.options.ssthresh = inflight / 2
		if k.options.ssthresh < KCP_THRESH_MIN {
			k.options.ssthresh = KCP_THRESH_MIN
		}
		k.cwnd = k.options.ssthresh + resent
		k.incr = k.cwnd * k.mss
	}

	if lost {
		k.options.ssthresh = cwnd / 2
		if k.options.ssthresh < KCP_THRESH_MIN {
			k.options.ssthresh = KCP_THRESH_MIN
		}
		k.cwnd = 1
		k.incr = k.mss
	}

	if k.cwnd < 1 {
		k.cwnd = 1
		k.incr = k.mss
	}
}

func (k *KcpCB) Update(current int32) {
	k.current = current
	if !k.updated {
		k.updated = true
		k.ts_flush = k.current
	}

	var slap = timeDiff(k.current, k.ts_flush)
	if slap >= 10000 || slap < -10000 {
		k.ts_flush = k.current
		slap = 0
	}

	if slap >= 0 {
		k.ts_flush += k.options.interval
		if timeDiff(k.current, k.ts_flush) >= 0 {
			k.ts_flush = k.current + k.options.interval
		}
		k.Flush()
	}
}

func (k *KcpCB) Check(current int32) int32 {
	var (
		ts_flush  int32 = k.ts_flush
		tm_flush  int32 = 0x7fffffff
		tm_packet int32 = 0x7fffffff
		minimal   int32 = 0
	)

	if !k.updated {
		return current
	}

	if timeDiff(current, ts_flush) >= 10000 || timeDiff(current, ts_flush) < -10000 {
		ts_flush = current
	}

	if timeDiff(current, ts_flush) >= 0 {
		return current
	}

	tm_flush = timeDiff(ts_flush, current)

	var iter = k.snd_buf.Begin()
	for iter != k.snd_buf.End() {
		var seg = iter.Value().(*segment)
		var diff = timeDiff(seg.resendts, current)
		if diff <= 0 {
			return current
		}
		if diff < tm_packet {
			tm_packet = diff
		}
		iter = iter.Next()
	}

	if tm_packet < tm_flush {
		minimal = tm_packet
	} else {
		minimal = tm_flush
	}

	if minimal >= k.options.interval {
		minimal = k.options.interval
	}

	return current + minimal
}

func (k *KcpCB) SetMtu(mtu int32) int32 {
	if mtu < 50 || mtu < KCP_OVERHEAD {
		return -1
	}

	if int32(k.options.mtu) == mtu {
		return 0
	}

	k.options.mtu = mtu
	k.mss = k.options.mtu - KCP_OVERHEAD
	k.buffer = make([]byte, (mtu+KCP_OVERHEAD)*3)
	return 0
}

func (k *KcpCB) SetInterval(interval int32) {
	if interval > 5000 {
		interval = 5000
	} else if interval < 10 {
		interval = 10
	}
	k.options.interval = interval
}

func (k *KcpCB) SetNodelay(nodelay, interval, resend int32, nc bool) {
	if nodelay >= 0 {
		k.options.nodelay = nodelay
		if nodelay > 0 {
			k.options.rx_minrto = KCP_RTO_NDL
		} else {
			k.options.rx_minrto = KCP_RTO_MIN
		}
	}
	if interval >= 0 {
		k.SetInterval(interval)
	}
	if resend >= 0 {
		k.options.fastresend = resend
	}
	k.options.nocwnd = nc
}

func (k *KcpCB) SetWndSize(sndWnd, rcvWnd int32) {
	if sndWnd > 0 {
		k.options.snd_wnd = sndWnd
	}
	if rcvWnd > 0 {
		k.options.rcv_wnd = rcvWnd
	}
}

func (k *KcpCB) GetWaitSnd() int32 {
	return k.snd_buf.GetLength() + k.snd_queue.GetLength()
}

func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func bound(a, b, c int32) int32 {
	return min(max(a, b), c)
}

func timeDiff(later, earlier int32) int32 {
	return later - earlier
}

func encode16(data []byte, value int16) {
	if KCP_BIGENDIAN {
		data[0] = byte(value & 0xff)
		data[1] = byte(value >> 8)
	} else {
		data[0] = byte(value >> 8)
		data[1] = byte(value & 0xff)
	}
}

func decode16(data []byte) int16 {
	if KCP_BIGENDIAN {
		return int16(data[0]) + int16(data[1])<<8
	} else {
		return int16(data[0])<<8 + int16(data[1])
	}
}

func encode32(data []byte, value int32) {
	if KCP_BIGENDIAN {
		data[0] = byte(value & 0xff)
		data[1] = byte(value >> 8)
		data[2] = byte(value >> 16)
		data[3] = byte(value >> 24)
	} else {
		data[0] = byte(value >> 24)
		data[1] = byte(value >> 16)
		data[2] = byte(value >> 8)
		data[3] = byte(value & 0xff)
	}
}

func decode32(data []byte) int32 {
	if KCP_BIGENDIAN {
		return int32(data[0]) + int32(data[1])<<8&0xff00 + int32(data[2])<<16&0x00ff0000 + int32(data[3])<<24&0x7f000000
	} else {
		return int32(data[0])<<24&0x7f000000 + int32(data[1])<<16&0x00ff0000 + int32(data[2])<<8&0xff00 + int32(data[3])
	}
}

var (
	segmentPool *sync.Pool
)

func init() {
	segmentPool = &sync.Pool{
		New: func() any {
			return &segment{}
		},
	}
}

func getSeg() *segment {
	return segmentPool.Get().(*segment)
}

func putSeg(seg *segment) {
	seg.clear()
	segmentPool.Put(seg)
}
