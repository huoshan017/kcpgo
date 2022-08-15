package kcp

type Options struct {
	mtu                int32
	rx_rto, rx_minrto  int32
	snd_wnd, rcv_wnd   int32
	nodelay            int32
	fastresend         int32
	nocwnd, stream     bool
	interval           int32
	ssthresh           int32
	fastlimit          int32
	dead_link          int32
	userfree_outputbuf bool
}

func (options Options) GetMtu() int32 {
	return options.mtu
}

func (options Options) GetMinRTO() int32 {
	return options.rx_minrto
}

func (options Options) GetRTO() int32 {
	return options.rx_rto
}

func (options Options) GetSendWnd() int32 {
	return options.snd_wnd
}

func (options Options) GetRecvWnd() int32 {
	return options.rcv_wnd
}

func (options Options) GetNodelay() int32 {
	return options.nodelay
}

func (options Options) GetFastResend() int32 {
	return options.fastresend
}

func (options Options) GetNoCwnd() bool {
	return options.nocwnd
}

func (options Options) GetStream() bool {
	return options.stream
}

func (options Options) GetInterval() int32 {
	return options.interval
}

func (options Options) GetSSThresh() int32 {
	return options.ssthresh
}

func (options Options) GetFastAckLimit() int32 {
	return options.fastlimit
}

func (options Options) GetDeadLink() int32 {
	return options.dead_link
}

func (options Options) IsUserFreeOutputBuf() bool {
	return options.userfree_outputbuf
}

func (options *Options) SetMtu(mtu int32) {
	options.mtu = mtu
}

func (options *Options) SetMinRTO(minrto int32) {
	options.rx_minrto = minrto
}

func (options *Options) SetRTO(rto int32) {
	options.rx_rto = rto
}

func (options *Options) SetSendWnd(wnd int32) {
	options.snd_wnd = wnd
}

func (options *Options) SetRecvWnd(wnd int32) {
	options.rcv_wnd = wnd
}

func (options *Options) SetNodelay(nodelay int32) {
	options.nodelay = nodelay
}

func (options *Options) SetFastResend(resend int32) {
	options.fastresend = resend
}

func (options *Options) SetNoCwnd(nocwnd bool) {
	options.nocwnd = nocwnd
}

func (options *Options) SetStream(stream bool) {
	options.stream = stream
}

func (options *Options) SetInterval(interval int32) {
	options.interval = interval
}

func (options *Options) SetSSThresh(ssthresh int32) {
	options.ssthresh = ssthresh
}

func (options *Options) SetFastAckLimit(limit int32) {
	options.fastlimit = limit
}

func (options *Options) SetDeadLink(deadlink int32) {
	options.dead_link = deadlink
}

func (options *Options) SetUserFreeOutputBuf(userfree bool) {
	options.userfree_outputbuf = userfree
}

type Option func(*Options)

func WithStream(stream bool) Option {
	return func(options *Options) {
		options.stream = stream
	}
}

func WithMtu(mtu int32) Option {
	return func(options *Options) {
		if mtu > KCP_MTU_MAX {
			mtu = KCP_MTU_MAX
		}
		options.mtu = mtu
	}
}

func WithRTO(rto int32) Option {
	return func(options *Options) {
		options.rx_rto = rto
	}
}

func WithMinRTO(rto int32) Option {
	return func(options *Options) {
		options.rx_minrto = rto
	}
}

func WithWnd(sndWnd, rcvWnd int32) Option {
	return func(options *Options) {
		options.snd_wnd = sndWnd
		options.rcv_wnd = rcvWnd
	}
}

func WithNodelay(nodelay int32) Option {
	return func(options *Options) {
		options.nodelay = nodelay
	}
}

func WithFastResend(fastresend int32) Option {
	return func(options *Options) {
		options.fastresend = fastresend
	}
}

func WithNoCwnd(nocwnd bool) Option {
	return func(options *Options) {
		options.nocwnd = nocwnd
	}
}

func WithInterval(interval int32) Option {
	return func(options *Options) {
		options.interval = interval
	}
}

func WithSSThresh(ssthresh int32) Option {
	return func(options *Options) {
		options.ssthresh = ssthresh
	}
}

func WithFastAckLimit(fastlimit int32) Option {
	return func(options *Options) {
		options.fastlimit = fastlimit
	}
}

func WithDeadLink(deadlink int32) Option {
	return func(options *Options) {
		options.dead_link = deadlink
	}
}

func WithUserFreeOutputBuf(userfree bool) Option {
	return func(options *Options) {
		options.userfree_outputbuf = userfree
	}
}
