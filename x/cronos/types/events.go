package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	AttributeValueCategory = ModuleName

	AttributeKeyRecipient             = "recipient"
	AttributeKeySender                = "sender"
	AttributeKeyAmount                = "amount"
	AttributeKeyReceiver              = "receiver"
	AttributeKeyEthereumTokenContract = "ethereum_token_contract"

	EventTypeConvertVouchers             = "convert_vouchers"
	EventTypeTransferTokens              = "transfer_tokens"
	EventTypeEthereumSendToCosmosHandled = "ethereum_send_to_cosmos_handled"
	// EventTypeRefundVoucherConversionFailed is emitted when IBC refund tokens were returned
	// but EVM-side voucher conversion failed (ack error or timeout refund path).
	EventTypeRefundVoucherConversionFailed = "refund_voucher_conversion_failed"

	RefundPathAcknowledgement = "acknowledgement"
	RefundPathTimeout         = "timeout"

	AttributeKeyDenom       = "denom"
	AttributeKeyRefundPath  = "refund_path"
	AttributeKeyErrorString = "error"
)

// NewConvertVouchersEvent constructs a new voucher convert sdk.Event
// nolint: interfacer
func NewConvertVouchersEvent(sender string, amount fmt.Stringer) sdk.Event {
	return sdk.NewEvent(
		EventTypeConvertVouchers,
		sdk.NewAttribute(AttributeKeySender, sender),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}

// NewTransferTokensEvent constructs a new transfer sdk.Event
func NewTransferTokensEvent(sender, recipient string, amount fmt.Stringer) sdk.Event {
	return sdk.NewEvent(
		EventTypeTransferTokens,
		sdk.NewAttribute(AttributeKeySender, sender),
		sdk.NewAttribute(AttributeKeyRecipient, recipient),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}

// NewRefundVoucherConversionFailedEvent is emitted when refund voucher→EVM conversion fails
// after the transfer module has already credited the refunded native coins.
func NewRefundVoucherConversionFailedEvent(refundPath, denom, errStr string) sdk.Event {
	return sdk.NewEvent(
		EventTypeRefundVoucherConversionFailed,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(AttributeKeyRefundPath, refundPath),
		sdk.NewAttribute(AttributeKeyDenom, denom),
		sdk.NewAttribute(AttributeKeyErrorString, errStr),
	)
}
