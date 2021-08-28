package types

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	AttributeValueCategory = ModuleName

	AttributeKeyRecipient = "recipient"
	AttributeKeySender    = "sender"

	// events
	EventTypeConvertCoin = "convert_coin"
)

// NewCoinSpentEvent constructs a new coin convert sdk.Event
// nolint: interfacer
func NewConvertCoinEvent(sender sdk.AccAddress, amount sdk.Coins) sdk.Event {
	return sdk.NewEvent(
		EventTypeConvertCoin,
		sdk.NewAttribute(AttributeKeySender, sender.String()),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}
