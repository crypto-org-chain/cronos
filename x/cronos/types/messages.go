package types

import (
	"bytes"
	stderrors "errors"

	"filippo.io/age"
	"github.com/ethereum/go-ethereum/common"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgUpdateTokenMapping = "UpdateTokenMapping"

var (
	_ sdk.Msg = &MsgConvertVouchers{}
	_ sdk.Msg = &MsgTransferTokens{}
	_ sdk.Msg = &MsgUpdateTokenMapping{}
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgTurnBridge{}
	_ sdk.Msg = &MsgUpdatePermissions{}
	_ sdk.Msg = &MsgStoreBlockList{}
)

func NewMsgConvertVouchers(address string, coins sdk.Coins) *MsgConvertVouchers {
	return &MsgConvertVouchers{
		Address: address,
		Coins:   coins,
	}
}

// ValidateBasic ...
func (msg *MsgConvertVouchers) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address (%s)", err)
	}
	if !msg.Coins.IsValid() {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}

	if !msg.Coins.IsAllPositive() {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}
	return nil
}

var _ sdk.Msg = &MsgTransferTokens{}

func NewMsgTransferTokens(from, to string, coins sdk.Coins) *MsgTransferTokens {
	return &MsgTransferTokens{
		From:  from,
		To:    to,
		Coins: coins,
	}
}

// ValidateBasic ...
func (msg *MsgTransferTokens) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address address (%s)", err)
	}

	// TODO, validate TO address format

	if !msg.Coins.IsValid() {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}

	if !msg.Coins.IsAllPositive() {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}
	return nil
}

var _ sdk.Msg = &MsgUpdateTokenMapping{}

// NewMsgUpdateTokenMapping ...
func NewMsgUpdateTokenMapping(admin, denom, contract, symbol string, decimal uint32) *MsgUpdateTokenMapping {
	return &MsgUpdateTokenMapping{
		Sender:   admin,
		Denom:    denom,
		Contract: contract,
		Symbol:   symbol,
		Decimal:  decimal,
	}
}

// GetSigners ...
func (msg *MsgUpdateTokenMapping) GetSigners() []sdk.AccAddress {
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{sender}
}

// ValidateBasic ...
func (msg *MsgUpdateTokenMapping) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
	}

	if !IsValidCoinDenom(msg.Denom) {
		return errors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid denom format (%s)", msg.Denom)
	}

	if !common.IsHexAddress(msg.Contract) {
		return errors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid contract address (%s)", msg.Contract)
	}

	return nil
}

// Type ...
func (msg MsgUpdateTokenMapping) Type() string {
	return TypeMsgUpdateTokenMapping
}

// NewMsgTurnBridge ...
func NewMsgTurnBridge(admin string, enable bool) *MsgTurnBridge {
	return &MsgTurnBridge{
		Sender: admin,
		Enable: enable,
	}
}

// ValidateBasic ...
func (msg *MsgTurnBridge) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
	}

	return nil
}

func NewMsgUpdateParams(authority string, params Params) *MsgUpdateParams {
	return &MsgUpdateParams{
		Authority: authority,
		Params:    params,
	}
}

// ValidateBasic does a sanity check on the provided data.
func (msg *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errors.Wrap(err, "invalid authority address")
	}

	if err := msg.Params.Validate(); err != nil {
		return err
	}

	return nil
}

// NewMsgUpdatePermissions ...
func NewMsgUpdatePermissions(from, address string, permissions uint64) *MsgUpdatePermissions {
	return &MsgUpdatePermissions{
		From:        from,
		Address:     address,
		Permissions: permissions,
	}
}

// ValidateBasic ...
func (msg *MsgUpdatePermissions) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
	}
	_, err = sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid target address (%s)", err)
	}

	return nil
}

func NewMsgStoreBlockList(from string, blob []byte) *MsgStoreBlockList {
	return &MsgStoreBlockList{
		From: from,
		Blob: blob,
	}
}

var errDummyIdentity = stderrors.New("dummy")

type dummyIdentity struct{}

func (i *dummyIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	return nil, errDummyIdentity
}

func (msg *MsgStoreBlockList) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
	}
	// skip heavy operation in Decrypt by early return with errDummyIdentity in
	// https://github.com/FiloSottile/age/blob/v1.1.1/age.go#L197
	_, err = age.Decrypt(bytes.NewBuffer(msg.Blob), new(dummyIdentity))
	if err != nil && !stderrors.Is(err, errDummyIdentity) {
		return err
	}
	return nil
}
