# Changelog

## Unreleased

*November 30, 2021*

## v0.6.4

### Bug Fixes
- [crypto-org-chain/ethermint#19](https://github.com/crypto-org-chain/ethermint/pull/19) revert tharsis#786 because it contains consensus breaking changes

*November 29, 2021*

## v0.6.3

### Bug Fixes

- [tharsis#781](https://github.com/tharsis/ethermint/pull/781) fix empty transactions in getBlock
- [crypto-org-chain/ethermint#15](https://github.com/crypto-org-chain/ethermint/pull/15) web3 rpc api returns wrong block gas limit
- [crypto-org-chain/ethermint#16](https://github.com/crypto-org-chain/ethermint/pull/16) fix unwrap context panic in BlockMaxGasFromConsensusParams

### Improvements

- [tharsis#786](https://github.com/tharsis/ethermint/pull/786) Improve error message of `SendTransaction`/`SendRawTransaction` JSON-RPC APIs.
- [cronos#222](https://github.com/crypto-org-chain/cronos/pull/222) change solc 0.6.11 to 0.6.8 (from dapp cachix) and update hermes to 0.8.

*November 19, 2021*

## v0.6.2

### Bug Fixes
- [tharsis#720](https://github.com/tharsis/ethermint/pull/720) traceTransaction fails for succesful tx
- [tharsis#743](https://github.com/tharsis/ethermint/pull/743) missing debug_tranceBlockByHash RPC method and fix debug_traceBlock*
- [tharsis#746](https://github.com/tharsis/ethermint/pull/746) set debug based on tracer
- [tharsis#741](https://github.com/tharsis/ethermint/pull/741) filter non eth txs in block rpc response
- [crypto-org-chain/ethermint#12](https://github.com/crypto-org-chain/ethermint/pull/12) reject tx with too large gas limit


*October 26, 2021*

## v0.6.1

### State Machine Breaking
- [cronos#190](https://github.com/crypto-org-chain/cronos/pull/190) upgrade ethermint to v0.7.2 with (#661) and (#689)

### Bug Fixes
- [cronos#187](https://github.com/crypto-org-chain/cronos/pull/187) multiple denoms can be mapped to same contract
- [cronos#157](https://github.com/crypto-org-chain/cronos/pull/185) cronos params name has an unnecessary Key prefix
- [cronos#179](https://github.com/crypto-org-chain/cronos/pull/179) fix denom (symbol) in CRC20Module
- [cronos#178](https://github.com/crypto-org-chain/cronos/pull/178) version CLI command doesn't output any text



*October 13, 2021*

## v0.6.0

This version removes gravity-bridge from cronos, also includes multiple bug fixes in third-party dependencies.

### Consensus breaking changes

- [cronos#171](https://github.com/crypto-org-chain/cronos/pull/171) remove gravity-bridge for mainnet launch

### Bug Fixes
- [cronos#144](https://github.com/crypto-org-chain/cronos/pull/144) fix events in autodeploy crc20 module contract
- [gravity-bridge#17](https://github.com/crypto-org-chain/gravity-bridge/pull/17) processEthereumEvent does not persist hooks emitted event
- [gravity-bridge#20](https://github.com/crypto-org-chain/gravity-bridge/pull/20) fix undeterministic in consensus
- [cronos#167](https://github.com/crypto-org-chain/cronos/pull/167) upgrade cosmos-sdk to 0.44.2

### Improvements
- [cronos#162](https://github.com/crypto-org-chain/cronos/pull/162) bump ibc-go to v1.2.1 with hooks support
- [cronos#169](https://github.com/crypto-org-chain/cronos/pull/169) bump ethermint to v0.7.1 and go-ethereum to v10.1.3-patched which include (CVE-2021-39137) hotfix

*October 4, 2021*
## v0.5.5

This version fixes various bugs regarding ibc fund transfer and EVM-related in ethermint.
We also enable swagger doc ui and add the token mapping state in genesis.

### Bug Fixes

- [cronos#109](https://github.com/crypto-org-chain/cronos/issues/109) ibc transfer timeout too short
- [tharsis#590](https://github.com/tharsis/ethermint/pull/590) fix export contract state in genesis and reimport
- [cronos#123](https://github.com/crypto-org-chain/cronos/issues/123) fix ibc refund logic
- [tharsis#617](https://github.com/tharsis/ethermint/pull/617) iterator on deeply nested cache contexts is extremely slow
- [tharsis#615](https://github.com/tharsis/ethermint/pull/615) tx log attribtue value not parsable by some client

### Features

- [cronos#110](https://github.com/crypto-org-chain/cronos/pull/110) embed swagger doc ui
- [cronos#113](https://github.com/crypto-org-chain/cronos/pull/113) export token mapping state to genesis
- [cronos#128](https://github.com/crypto-org-chain/cronos/pull/128) add native message to update token mapping

*September 22, 2021*
## v0.5.4

This version is the same as v0.5.3 with a patched version of ethermint which include a bug fix on the transaction receipts events and on concurrent query.

### Bug Fixes

- [cronos#93](https://github.com/crypto-org-chain/cronos/pull/93) tx receipts don't contain events
- [cronos#98](https://github.com/crypto-org-chain/cronos/pull/98) node crash under concurrent query

*September 21, 2021*
## v0.5.3

This version contains several new features, it enables gravity bridge in Cronos and automatic token conversion for bridging tokens to crc20 tokens. It also fix the decimal conversion issues in the CRO tokens from crypto.org chain.
In addition to that, it also upgrade ethermint to its latest version (v0.5.0.x) which bring several breaking changes (see [changelog](https://github.com/tharsis/ethermint/blob/1a01c6a992c0fb94d70bb1c7127715874cefd057/CHANGELOG.md)).

### Consensus breaking changes
- [cronos#87](https://github.com/crypto-org-chain/cronos/pull/87) upgrade ethermint to v0.4.2-0.20210920104419-1a01c6a992c0

### Features

- [cronos#11](https://github.com/crypto-org-chain/cronos/pull/11) embed gravity bridge module
- [cronos#35](https://github.com/crypto-org-chain/cronos/pull/35) add support for ibc hook
- [cronos#55](https://github.com/crypto-org-chain/cronos/pull/55) add support for ibc token conversion to crc20
- [cronos#45](https://github.com/crypto-org-chain/cronos/pull/45) allow evm contract to call bank send and gravity send
- [cronos#65](https://github.com/crypto-org-chain/cronos/pull/65) support SendToIbc in evm_log_handlers
- [cronos#59](https://github.com/crypto-org-chain/cronos/pull/59) gravity bridged tokens are converted to crc20
  automatically
- [cronos#68](https://github.com/crypto-org-chain/cronos/issues/68) support SendCroToIbc in evm_log_handlers
- [cronos#86](https://github.com/crypto-org-chain/cronos/issues/86) change account prefix

*August 19, 2021*

## v0.5.2

### Consensus breaking changes

- (ethermint) [tharsis#447](https://github.com/tharsis/ethermint/pull/447) update `chain-id` format.

### Improvements

- (ethermint) [tharsis#434](https://github.com/tharsis/ethermint/pull/434) configurable vm tracer

### Bug Fixes

- (ethermint) [tharsis#446](https://github.com/tharsis/ethermint/pull/446) fix chain state export issue



*August 16, 2021*

## v0.5.1

This version is a new scaffolding of cronos project where ethermint is included as a library.

### Consensus breaking changes

- (ethermint) [tharsis#399](https://github.com/tharsis/ethermint/pull/399) Exception in sub-message call reverts the call if it's not propagated.
- (ethermint) [tharsis#334](https://github.com/tharsis/ethermint/pull/334) Log index changed to the index in block rather than tx.
- (ethermint) [tharsis#342](https://github.com/tharsis/ethermint/issues/342) Don't clear balance when resetting the account.
- (ethermint) [tharsis#383](https://github.com/tharsis/ethermint/pull/383) `GetCommittedState` use the original context.

### Features

### Improvements

- (ethermint) [tharsis#425](https://github.com/tharsis/ethermint/pull/425) Support build on linux arm64
- (ethermint) [tharsis#423](https://github.com/tharsis/ethermint/pull/423) Bump to cosmos-sdk 0.43.0

### Bug Fixes

- (ethermint) [tharsis#428](https://github.com/tharsis/ethermint/pull/428) [tharsis#375](https://github.com/tharsis/ethermint/pull/375) Multiple web3 rpc api fixes.
