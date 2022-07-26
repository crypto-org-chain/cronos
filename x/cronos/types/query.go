package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

func (m ReplayBlockRequest) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	for _, msg := range m.Msgs {
		if err := msg.UnpackInterfaces(unpacker); err != nil {
			return err
		}
	}
	return nil
}
