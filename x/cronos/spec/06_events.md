<!--
order: 6
-->

# Events

The Cronos module emits the Cosmos SDK [events](./../../../docs/quickstart/events.md#sdk-and-tendermint-events) after a state execution. It can be expected that the type `message`, with an
attribute key of `action` will represent the first event for each message being processed as emitted
by the Cosmos SDK's `Baseapp` (i.e the basic application that implements Tendermint Core's ABCI
interface).

## MsgConvertVouchers

| Type             | Attribute Key | Attribute Value    |
| ---------------- | ------------- | ------------------ |
| convert_vouchers | `"sender"`    | `{bech32_address}` |
| convert_vouchers | `"amount"`    | `{amount}`         |
| message          | module        | cronos             |
| message          | action        | ConvertVouchers    |

## MsgTransferTokens

| Type            | Attribute Key | Attribute Value    |
| --------------- | ------------- | ------------------ |
| transfer_tokens | `"sender"`    | `{bech32_address}` |
| transfer_tokens | `"recipient"` | `{bech32_address}` |
| transfer_tokens | `"amount"`    | `{amount}`         |
| message         | module        | cronos             |
| message         | action        | TransferTokens     |

## MsgUpdateTokenMapping

| Type    | Attribute Key | Attribute Value    |
| ------- | ------------- | ------------------ |
| message | action        | UpdateTokenMapping |
