package types

import (
	fmt "fmt"

	"filippo.io/age"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = (*MsgRegisterEncryptionKey)(nil)

func (m *MsgRegisterEncryptionKey) ValidateBasic() error {
	// validate bech32 format of Address
	if _, err := sdk.AccAddressFromBech32(m.Address); err != nil {
		return fmt.Errorf("invalid address: %s", err)
	}
	return ValidateRecipientKey(m.Key)
}

func ValidateRecipientKey(key string) error {
	_, err := age.ParseX25519Recipient(key)
	return err
}
