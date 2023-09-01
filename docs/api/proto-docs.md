<!-- This file is auto-generated. Please do not modify it yourself. -->
# Protobuf Documentation
<a name="top"></a>

## Table of Contents

- [cronos/cronos.proto](#cronos/cronos.proto)
    - [Params](#cronos.Params)
    - [TokenMapping](#cronos.TokenMapping)
    - [TokenMappingChangeProposal](#cronos.TokenMappingChangeProposal)
  
- [cronos/genesis.proto](#cronos/genesis.proto)
    - [GenesisState](#cronos.GenesisState)
  
- [cronos/query.proto](#cronos/query.proto)
    - [ContractByDenomRequest](#cronos.ContractByDenomRequest)
    - [ContractByDenomResponse](#cronos.ContractByDenomResponse)
    - [DenomByContractRequest](#cronos.DenomByContractRequest)
    - [DenomByContractResponse](#cronos.DenomByContractResponse)
    - [ReplayBlockRequest](#cronos.ReplayBlockRequest)
    - [ReplayBlockResponse](#cronos.ReplayBlockResponse)
  
    - [Query](#cronos.Query)
  
- [cronos/tx.proto](#cronos/tx.proto)
    - [MsgConvertVouchers](#cronos.MsgConvertVouchers)
    - [MsgConvertVouchersResponse](#cronos.MsgConvertVouchersResponse)
    - [MsgTransferTokens](#cronos.MsgTransferTokens)
    - [MsgTransferTokensResponse](#cronos.MsgTransferTokensResponse)
    - [MsgUpdateTokenMapping](#cronos.MsgUpdateTokenMapping)
    - [MsgUpdateTokenMappingResponse](#cronos.MsgUpdateTokenMappingResponse)
  
    - [Msg](#cronos.Msg)
  
- [icaauth/v1/params.proto](#icaauth/v1/params.proto)
    - [Params](#cronos.icaauth.v1.Params)
  
- [icaauth/v1/genesis.proto](#icaauth/v1/genesis.proto)
    - [GenesisState](#cronos.icaauth.v1.GenesisState)
  
- [icaauth/v1/query.proto](#icaauth/v1/query.proto)
    - [QueryInterchainAccountAddressRequest](#cronos.icaauth.v1.QueryInterchainAccountAddressRequest)
    - [QueryInterchainAccountAddressResponse](#cronos.icaauth.v1.QueryInterchainAccountAddressResponse)
    - [QueryParamsRequest](#cronos.icaauth.v1.QueryParamsRequest)
    - [QueryParamsResponse](#cronos.icaauth.v1.QueryParamsResponse)
  
    - [Query](#cronos.icaauth.v1.Query)
  
- [icaauth/v1/tx.proto](#icaauth/v1/tx.proto)
    - [MsgRegisterAccount](#cronos.icaauth.v1.MsgRegisterAccount)
    - [MsgRegisterAccountResponse](#cronos.icaauth.v1.MsgRegisterAccountResponse)
    - [MsgSubmitTx](#cronos.icaauth.v1.MsgSubmitTx)
    - [MsgSubmitTxResponse](#cronos.icaauth.v1.MsgSubmitTxResponse)
  
    - [Msg](#cronos.icaauth.v1.Msg)
  
- [Scalar Value Types](#scalar-value-types)



<a name="cronos/cronos.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cronos/cronos.proto



<a name="cronos.Params"></a>

### Params
Params defines the parameters for the cronos module.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `ibc_cro_denom` | [string](#string) |  |  |
| `ibc_timeout` | [uint64](#uint64) |  |  |
| `cronos_admin` | [string](#string) |  | the admin address who can update token mapping |
| `enable_auto_deployment` | [bool](#bool) |  |  |






<a name="cronos.TokenMapping"></a>

### TokenMapping
TokenMapping defines a mapping between native denom and contract


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `denom` | [string](#string) |  |  |
| `contract` | [string](#string) |  |  |






<a name="cronos.TokenMappingChangeProposal"></a>

### TokenMappingChangeProposal
TokenMappingChangeProposal defines a proposal to change one token mapping.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `title` | [string](#string) |  |  |
| `description` | [string](#string) |  |  |
| `denom` | [string](#string) |  |  |
| `contract` | [string](#string) |  |  |
| `symbol` | [string](#string) |  | only when updating cronos (source) tokens |
| `decimal` | [uint32](#uint32) |  |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="cronos/genesis.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cronos/genesis.proto



<a name="cronos.GenesisState"></a>

### GenesisState
GenesisState defines the cronos module's genesis state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `params` | [Params](#cronos.Params) |  | params defines all the paramaters of the module. |
| `external_contracts` | [TokenMapping](#cronos.TokenMapping) | repeated |  |
| `auto_contracts` | [TokenMapping](#cronos.TokenMapping) | repeated | this line is used by starport scaffolding # genesis/proto/state this line is used by starport scaffolding # ibc/genesis/proto |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="cronos/query.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cronos/query.proto



<a name="cronos.ContractByDenomRequest"></a>

### ContractByDenomRequest
ContractByDenomRequest is the request type of ContractByDenom call


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `denom` | [string](#string) |  |  |






<a name="cronos.ContractByDenomResponse"></a>

### ContractByDenomResponse
ContractByDenomRequest is the response type of ContractByDenom call


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `contract` | [string](#string) |  |  |
| `auto_contract` | [string](#string) |  |  |






<a name="cronos.DenomByContractRequest"></a>

### DenomByContractRequest
DenomByContractRequest is the request type of DenomByContract call


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `contract` | [string](#string) |  |  |






<a name="cronos.DenomByContractResponse"></a>

### DenomByContractResponse
DenomByContractResponse is the response type of DenomByContract call


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `denom` | [string](#string) |  |  |






<a name="cronos.ReplayBlockRequest"></a>

### ReplayBlockRequest
ReplayBlockRequest


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `msgs` | [ethermint.evm.v1.MsgEthereumTx](#ethermint.evm.v1.MsgEthereumTx) | repeated | the eth messages in the block |
| `block_number` | [int64](#int64) |  |  |
| `block_hash` | [string](#string) |  |  |
| `block_time` | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  |  |






<a name="cronos.ReplayBlockResponse"></a>

### ReplayBlockResponse
ReplayBlockResponse


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `responses` | [ethermint.evm.v1.MsgEthereumTxResponse](#ethermint.evm.v1.MsgEthereumTxResponse) | repeated |  |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="cronos.Query"></a>

### Query
Query defines the gRPC querier service.

| Method Name | Request Type | Response Type | Description | HTTP Verb | Endpoint |
| ----------- | ------------ | ------------- | ------------| ------- | -------- |
| `ContractByDenom` | [ContractByDenomRequest](#cronos.ContractByDenomRequest) | [ContractByDenomResponse](#cronos.ContractByDenomResponse) | ContractByDenom queries contract addresses by native denom | GET|/cronos/v1/contract_by_denom/{denom}|
| `DenomByContract` | [DenomByContractRequest](#cronos.DenomByContractRequest) | [DenomByContractResponse](#cronos.DenomByContractResponse) | DenomByContract queries native denom by contract address | GET|/cronos/v1/denom_by_contract/{contract}|
| `ReplayBlock` | [ReplayBlockRequest](#cronos.ReplayBlockRequest) | [ReplayBlockResponse](#cronos.ReplayBlockResponse) | ReplayBlock replay the eth messages in the block to recover the results of false-failed txs. | |

 <!-- end services -->



<a name="cronos/tx.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cronos/tx.proto



<a name="cronos.MsgConvertVouchers"></a>

### MsgConvertVouchers
MsgConvertVouchers represents a message to convert ibc voucher coins to cronos evm coins.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `address` | [string](#string) |  |  |
| `coins` | [cosmos.base.v1beta1.Coin](#cosmos.base.v1beta1.Coin) | repeated |  |






<a name="cronos.MsgConvertVouchersResponse"></a>

### MsgConvertVouchersResponse
MsgConvertVouchersResponse defines the ConvertVouchers response type.






<a name="cronos.MsgTransferTokens"></a>

### MsgTransferTokens
MsgTransferTokens represents a message to transfer cronos evm coins through ibc.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `from` | [string](#string) |  |  |
| `to` | [string](#string) |  |  |
| `coins` | [cosmos.base.v1beta1.Coin](#cosmos.base.v1beta1.Coin) | repeated |  |






<a name="cronos.MsgTransferTokensResponse"></a>

### MsgTransferTokensResponse
MsgTransferTokensResponse defines the TransferTokens response type.






<a name="cronos.MsgUpdateTokenMapping"></a>

### MsgUpdateTokenMapping
MsgUpdateTokenMapping defines the request type


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `sender` | [string](#string) |  |  |
| `denom` | [string](#string) |  |  |
| `contract` | [string](#string) |  |  |
| `symbol` | [string](#string) |  | only when updating cronos (source) tokens |
| `decimal` | [uint32](#uint32) |  |  |






<a name="cronos.MsgUpdateTokenMappingResponse"></a>

### MsgUpdateTokenMappingResponse
MsgUpdateTokenMappingResponse defines the response type





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="cronos.Msg"></a>

### Msg
Msg defines the Cronos Msg service

| Method Name | Request Type | Response Type | Description | HTTP Verb | Endpoint |
| ----------- | ------------ | ------------- | ------------| ------- | -------- |
| `ConvertVouchers` | [MsgConvertVouchers](#cronos.MsgConvertVouchers) | [MsgConvertVouchersResponse](#cronos.MsgConvertVouchersResponse) | ConvertVouchers defines a method for converting ibc voucher to cronos evm coins. | |
| `TransferTokens` | [MsgTransferTokens](#cronos.MsgTransferTokens) | [MsgTransferTokensResponse](#cronos.MsgTransferTokensResponse) | TransferTokens defines a method to transfer cronos evm coins to another chain through IBC | |
| `UpdateTokenMapping` | [MsgUpdateTokenMapping](#cronos.MsgUpdateTokenMapping) | [MsgUpdateTokenMappingResponse](#cronos.MsgUpdateTokenMappingResponse) | UpdateTokenMapping defines a method to update token mapping | |

 <!-- end services -->



<a name="icaauth/v1/params.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## icaauth/v1/params.proto



<a name="cronos.icaauth.v1.Params"></a>

### Params
Params defines the parameters for the module.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `min_timeout_duration` | [google.protobuf.Duration](#google.protobuf.Duration) |  | min_timeout_duration defines the minimum value of packet timeout when submitting transactions to host chain on behalf of interchain account |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="icaauth/v1/genesis.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## icaauth/v1/genesis.proto



<a name="cronos.icaauth.v1.GenesisState"></a>

### GenesisState
GenesisState defines the icaauth module's genesis state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `params` | [Params](#cronos.icaauth.v1.Params) |  | params defines the genesis parameters |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->

 <!-- end services -->



<a name="icaauth/v1/query.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## icaauth/v1/query.proto



<a name="cronos.icaauth.v1.QueryInterchainAccountAddressRequest"></a>

### QueryInterchainAccountAddressRequest
QueryInterchainAccountAddressRequest defines the request for the InterchainAccountAddress query.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `connection_id` | [string](#string) |  |  |
| `owner` | [string](#string) |  |  |






<a name="cronos.icaauth.v1.QueryInterchainAccountAddressResponse"></a>

### QueryInterchainAccountAddressResponse
QueryInterchainAccountAddressResponse defines the response for the InterchainAccountAddress query.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `interchain_account_address` | [string](#string) |  |  |






<a name="cronos.icaauth.v1.QueryParamsRequest"></a>

### QueryParamsRequest
QueryParamsRequest is request type for the Query/Params RPC method.






<a name="cronos.icaauth.v1.QueryParamsResponse"></a>

### QueryParamsResponse
QueryParamsResponse is response type for the Query/Params RPC method.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `params` | [Params](#cronos.icaauth.v1.Params) |  | params holds all the parameters of this module. |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="cronos.icaauth.v1.Query"></a>

### Query
Query defines the gRPC querier service.

| Method Name | Request Type | Response Type | Description | HTTP Verb | Endpoint |
| ----------- | ------------ | ------------- | ------------| ------- | -------- |
| `Params` | [QueryParamsRequest](#cronos.icaauth.v1.QueryParamsRequest) | [QueryParamsResponse](#cronos.icaauth.v1.QueryParamsResponse) | Parameters queries the parameters of the module. | GET|/cronos/icaauth/v1/params|
| `InterchainAccountAddress` | [QueryInterchainAccountAddressRequest](#cronos.icaauth.v1.QueryInterchainAccountAddressRequest) | [QueryInterchainAccountAddressResponse](#cronos.icaauth.v1.QueryInterchainAccountAddressResponse) | InterchainAccountAddress queries the interchain account address for given `connection_id` and `owner` | GET|/cronos/icaauth/v1/interchain_account_address/{connection_id}/{owner}|

 <!-- end services -->



<a name="icaauth/v1/tx.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## icaauth/v1/tx.proto



<a name="cronos.icaauth.v1.MsgRegisterAccount"></a>

### MsgRegisterAccount
MsgRegisterAccount defines the request message for MsgRegisterAccount


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `owner` | [string](#string) |  | owner represents the owner of the interchain account |
| `connection_id` | [string](#string) |  | connection_id represents the IBC `connection_id` of the host chain |
| `version` | [string](#string) |  | version represents the version of the ICA channel |






<a name="cronos.icaauth.v1.MsgRegisterAccountResponse"></a>

### MsgRegisterAccountResponse
MsgRegisterAccountResponse defines the response message for MsgRegisterAccount






<a name="cronos.icaauth.v1.MsgSubmitTx"></a>

### MsgSubmitTx
MsgSubmitTx defines the request message for MsgSubmitTx


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `owner` | [string](#string) |  | owner represents the owner of the interchain account |
| `connection_id` | [string](#string) |  | connection_id represents the IBC `connection_id` of the host chain |
| `msgs` | [google.protobuf.Any](#google.protobuf.Any) | repeated | msgs represents the transactions to be submitted to the host chain |
| `timeout_duration` | [google.protobuf.Duration](#google.protobuf.Duration) |  | timeout_duration represents the timeout duration for the IBC packet from last block |






<a name="cronos.icaauth.v1.MsgSubmitTxResponse"></a>

### MsgSubmitTxResponse
MsgSubmitTxResponse defines the response message for MsgSubmitTx





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="cronos.icaauth.v1.Msg"></a>

### Msg
Msg defines the Msg service.

| Method Name | Request Type | Response Type | Description | HTTP Verb | Endpoint |
| ----------- | ------------ | ------------- | ------------| ------- | -------- |
| `RegisterAccount` | [MsgRegisterAccount](#cronos.icaauth.v1.MsgRegisterAccount) | [MsgRegisterAccountResponse](#cronos.icaauth.v1.MsgRegisterAccountResponse) | RegisterAccount registers an interchain account on host chain with given `connection_id` | |
| `SubmitTx` | [MsgSubmitTx](#cronos.icaauth.v1.MsgSubmitTx) | [MsgSubmitTxResponse](#cronos.icaauth.v1.MsgSubmitTxResponse) | SubmitTx submits a transaction to the host chain on behalf of interchain account | |

 <!-- end services -->



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

