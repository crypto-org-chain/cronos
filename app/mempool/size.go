package mempool

import "math/bits"

// ProtoSizeForTx returns the wire size a single tx contributes to a CometBFT
// block's Data message. It is identical to
// cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{bz}) but without the per-call
// []Tx slice + Data{}.ToProto allocation (~4 allocs/tx) that call makes in the
// proposal hot loop. Data.Txs is `repeated bytes txs = 1`; each element encodes
// as a 1-byte field tag (field 1, wire type 2) + a varint length + the payload,
// matching gogoproto's generated Size().
func ProtoSizeForTx(bz []byte) int64 {
	l := len(bz)
	return int64(1 + sovLen(uint64(l)) + l)
}

// sovLen returns the number of bytes gogoproto uses to encode x as a varint,
// matching the generated sov* helpers (7 bits per byte).
func sovLen(x uint64) int {
	return (bits.Len64(x|1) + 6) / 7
}
