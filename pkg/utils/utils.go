package utils

const (
	InfoHashLen   = 20
	PieceHashLen  = 20
	PeerIDLen     = 20
	ProtocolIDLen = 19
	ProtocolID    = "BitTorrent protocol"
	HandshakeSize = 1 + ProtocolIDLen + 8 + InfoHashLen + PeerIDLen
)
