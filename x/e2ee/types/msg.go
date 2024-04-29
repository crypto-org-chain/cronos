package types

import (
	fmt "fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (m *MsgRegisterEncryptionKey) ValidateBasic() error {
	if m.Address == "" {
		return fmt.Errorf("address cannot be empty")
	}
	if len(m.Key) == 0 {
		return fmt.Errorf("key cannot be nil")
	}
	// validate bech32 format of Address
	if _, err := sdk.AccAddressFromBech32(m.Address); err != nil {
		return fmt.Errorf("invalid address: %s", err)
	}
	return nil
}
