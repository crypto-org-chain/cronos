package mempool

import "math/bits"

// ProtoSizeForTx returns the wire size one tx contributes to a CometBFT block's
// Data message. Same result as cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{bz})
// but without its ~4 allocs/tx ([]Tx slice + Data{}.ToProto) in the proposal hot
// loop. Data.Txs is `repeated bytes txs = 1`: each element is a 1-byte field tag
// + varint length + payload, matching gogoproto's generated Size().
func ProtoSizeForTx(bz []byte) int64 {
	l := len(bz)
	return int64(1 + sovLen(uint64(l)) + l)
}

// sovLen returns the byte count gogoproto uses to varint-encode x, matching the
// generated sov* helpers (7 bits per byte).
func sovLen(x uint64) int {
	return (bits.Len64(x|1) + 6) / 7
}
