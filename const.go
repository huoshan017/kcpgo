package kcp

const (
	KCP_LOG_OUTPUT    = 1
	KCP_LOG_INPUT     = 2
	KCP_LOG_SEND      = 4
	KCP_LOG_RECV      = 8
	KCP_LOG_IN_DATA   = 16
	KCP_LOG_IN_ACK    = 32
	KCP_LOG_IN_PROBE  = 64
	KCP_LOG_IN_WINS   = 128
	KCP_LOG_OUT_DATA  = 256
	KCP_LOG_OUT_ACK   = 512
	KCP_LOG_OUT_PROBE = 1024
	KCP_LOG_OUT_WINS  = 2048
)

const (
	KCP_RTO_NDL             = 30
	KCP_RTO_MIN             = 100
	KCP_RTO_DEF             = 200
	KCP_RTO_MAX             = 60000
	KCP_CMD_PUSH      int32 = 81
	KCP_CMD_ACK       int32 = 82
	KCP_CMD_WASK      int32 = 83
	KCP_CMD_WINS      int32 = 84
	KCP_ASK_SEND            = 1
	KCP_ASK_TELL            = 2
	KCP_WND_SND             = 32
	KCP_WND_RCV             = 128
	KCP_MTU_DEF             = 1400
	KCP_ACK_FAST            = 3
	KCP_INTERVAL            = 100
	KCP_OVERHEAD            = 24
	KCP_DEADLINK            = 20
	KCP_THRESH_INIT         = 2
	KCP_THRESH_MIN          = 2
	KCP_PROBE_INIT          = 7000
	KCP_PROBE_LIMIT         = 120000
	KCP_FASTACK_LIMIT       = 5
)

const (
	KCP_FASTACK_CONSERVE = false
	KCP_BIGENDIAN        = true
)
