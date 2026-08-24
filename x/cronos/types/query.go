package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// MaxReplayBlockMsgs caps eth messages per ReplayBlock query. Also enforced
// in ReplayBlockRequest.Unmarshal (query.pb.go) to stop decode-time OOM —
// re-add that check by hand if query.pb.go is regenerated.
const MaxReplayBlockMsgs = 10000

func (m ReplayBlockRequest) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	for _, msg := range m.Msgs {
		if err := msg.UnpackInterfaces(unpacker); err != nil {
			return err
		}
	}
	return nil
}
