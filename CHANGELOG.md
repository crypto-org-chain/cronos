# Changelog

*Jan 05, 2024*

## v1.0.15

- [#1265](https://github.com/crypto-org-chain/cronos/pull/1265) Fix nil pointer panic when filter timeout
- [#1270](https://github.com/crypto-org-chain/cronos/pull/1270) Avoid out of bound panic from error message

*Dec 15, 2023*

## v1.0.14

- [#1259](https://github.com/crypto-org-chain/cronos/pull/1259) Use a hard-fork style upgrade to adjust feemarket parameters.

*Nov 20, 2023*

## v1.0.13

- [#1197](https://github.com/crypto-org-chain/cronos/pull/1197) tune rocksdb options to control memory consumption.
- [#1207](https://github.com/crypto-org-chain/cronos/pull/1207) Update rocksdb to `v8.6.7`.
- [#1240](https://github.com/crypto-org-chain/cronos/pull/1240) Revert rocksdb upgrade.
- [#1239](https://github.com/crypto-org-chain/cronos/pull/1239) Refactor websocket/subscription system to improve performance and stability.
- [#1246](https://github.com/crypto-org-chain/cronos/pull/1246) Update memiavl deps to include bug fixes and state sync restore performance improvement.

*Aug 11, 2023*

## v1.0.12

- [#986](https://github.com/crypto-org-chain/cronos/pull/986) Use go 1.20.
- [#984](https://github.com/crypto-org-chain/cronos/pull/984) experimental integration of memiavl.
- [#985](https://github.com/crypto-org-chain/cronos/pull/985) Fix versiondb verify command on older versions
- [#1043](https://github.com/crypto-org-chain/cronos/pull/1043) Integrate latest memiavl and local state-sync commands in cosmos-sdk
- [#1043](https://github.com/crypto-org-chain/cronos/pull/1043) Update ethermint dependency
  - Avoid channel get changed when concurrent subscribe happens ([ethermint commit](https://github.com/crypto-org-chain/ethermint/commit/72bbe0a80dfd3c586868e2f0b4fbed72593c45bf)).
  - call Close method on app to cleanup resource on graceful shutdown ([ethermint commit](https://github.com/crypto-org-chain/ethermint/commit/0ea7b86532a1144f229961f94b4524d5889e874d)).
- [#1081](https://github.com/crypto-org-chain/cronos/pull/1081) Build with nativebyteorder by default, the released binaries only support little-endian machines, big-endian machines need to build custom binary for themselves.
- [#940](https://github.com/crypto-org-chain/cronos/pull/940) Update rocksdb dependency to 8.1.1.
- [#1113](https://github.com/crypto-org-chain/cronos/pull/1113) Use standalone versiondb package, which supports restore from local snapshot.

### Bug Fixes

- [#1058](https://github.com/crypto-org-chain/cronos/pull/1058) Fix decode log for multi topics in websocket subscribe ([ethermint commit](https://github.com/crypto-org-chain/ethermint/commit/2136ad029860c819942ad1836dd3f42585002233)).
- [#1062](https://github.com/crypto-org-chain/cronos/pull/1062) Update cometbft `v0.34.29` with several minor bug fixes and low-severity security-fixes.
- [#1102](https://github.com/crypto-org-chain/cronos/pull/1102) avoid duplicate cache events emitted from ibc and gravity hook.
- [#1125](https://github.com/crypto-org-chain/cronos/pull/1125) Fix genesis migrate for feeibc, evm, feemarket and gravity.

*Jun 9, 2023*

## v1.0.9

- [#1059](https://github.com/crypto-org-chain/cronos/pull/1059) Patch barberry.

*May 30, 2023*

## v1.0.8

- [#1038](https://github.com/crypto-org-chain/cronos/pull/1038) Update ibc-go to `v5.2.1`.
- [#1052](https://github.com/crypto-org-chain/cronos/pull/1052) Revert accidental breaking change in `v1.0.7`.

*April 13, 2023*

## v1.0.7

### Improvements

- [#936](https://github.com/crypto-org-chain/cronos/pull/936) Reuse recovered sender address to optimize performance ([ethermint commit](https://github.com/crypto-org-chain/ethermint/commit/cb741e1d819683795aa32e286d31d8155f903cae)).
- [#949](https://github.com/crypto-org-chain/cronos/pull/949) Release static-linked binaries for linux platform.
- [#934](https://github.com/crypto-org-chain/cronos/pull/934) Add pebbledb backend.

### Bug Fixes

* [#953](https://github.com/crypto-org-chain/cronos/pull/953) Include third-party bug fixes:
  - update ethermint to include two bug fixes
    - https://github.com/crypto-org-chain/ethermint/pull/234
    - https://github.com/crypto-org-chain/ethermint/pull/233
  - update cosmos-sdk to include one bug fix
    - https://github.com/cosmos/cosmos-sdk/pull/15667
* [#945](https://github.com/crypto-org-chain/cronos/pull/945) Fix no handler exists for proposal type error when update-client due to wrong ibc route.

*Mar 16, 2023*

## v1.0.6

### Bug Fixes

* [#932](https://github.com/crypto-org-chain/cronos/pull/932) Backport multiple json-rpc bug fixes in ethermint ([commits](https://github.com/crypto-org-chain/ethermint/compare/v0.20.8-cronos...v0.20.9-cronos)).

*Mar 6, 2023*

## v1.0.5

### Bug Fixes

- [#908](https://github.com/crypto-org-chain/cronos/pull/908) Forbids negative priority fee.

### Improvements

- [#904](https://github.com/crypto-org-chain/cronos/pull/904) Enable "dynamic-level-bytes" on new `application.db`.
- [#907](https://github.com/crypto-org-chain/cronos/pull/907) Apply a configurable limit in rpc apis.
- [#909](https://github.com/crypto-org-chain/cronos/pull/909) Update to cosmos-sdk v0.46.11.

*Feb 15, 2023*

## v1.0.4

### Bug Fixes

- [#814](https://github.com/crypto-org-chain/cronos/pull/814) Fix prometheus metrics.
- [#857](https://github.com/crypto-org-chain/cronos/pull/857) Fix block hash in block filters.

### Improvements

- [#813](https://github.com/crypto-org-chain/cronos/pull/813) Tune up rocksdb options.
- [#791](https://github.com/crypto-org-chain/cronos/pull/791) Implement versiondb and migration commands.
- [#779](https://github.com/crypto-org-chain/cronos/pull/779) Add config `iavl-lazy-loading` to enable lazy loading of iavl store.
- [#871](https://github.com/crypto-org-chain/cronos/pull/871) Only ingest sst files to level 3 in versiondb migration.

*Feb 08, 2023*

## v1.0.3

### Bug Fixes

- [#846](https://github.com/crypto-org-chain/cronos/pull/846) Disable authz message

*Jan 04, 2023*

## v1.0.2

### State Machine Breaking

- [#802](https://github.com/crypto-org-chain/cronos/pull/802) Update ibc-go to `v5.2.0`.

*December 14, 2022*

## v1.0.1

### Improvements

- [#781](https://github.com/crypto-org-chain/cronos/pull/781) Add prune command.
- [#790](https://github.com/crypto-org-chain/cronos/pull/790) Update cosmos-sdk to `v0.46.7`, it fix a migration issue which affects pending proposals's votes during upgrade,
  it also adds the config entries for file streamer.

*Nov 22, 2022*

## v1.0.0

### Improvements

- [#772](https://github.com/crypto-org-chain/cronos/pull/772) Update cosmos-sdk to `v0.46.6`, it's non-breaking for cronos.

*Nov 17, 2022*

## v1.0.0-rc4

### Bug Fixes

- [#771](https://github.com/crypto-org-chain/cronos/pull/771) Fix london hardfork number in testnet3 parameters.

*Nov 13, 2022*

## v1.0.0-rc3

### State Machine Breaking

- [#765](https://github.com/crypto-org-chain/cronos/pull/765) Upgrade ibc-go to [v5.1.0](https://github.com/cosmos/ibc-go/releases/tag/v5.1.0) and related dependencies.

*Nov 10, 2022*

## v1.0.0-rc2

### Bug Fixes

- [#761](https://github.com/crypto-org-chain/cronos/pull/761) Fix non-deterministic evm execution result when there are concurrent grpc queries.
- [#762](https://github.com/crypto-org-chain/cronos/pull/762) Add `v1.0.0` upgrade plan for dry-run and mainnet upgrade, which clears the `extra_eips` parameter.
- [#763](https://github.com/crypto-org-chain/cronos/pull/763) Add error log for iavl set error.
- [#764](https://github.com/crypto-org-chain/cronos/pull/764) Make `eth_getProof` result compatible with ethereum.

*Nov 4, 2022*

## v1.0.0-rc1

### Bug Fixes

- [#760](https://github.com/crypto-org-chain/cronos/pull/760) Revert breaking changes on gas used in Ethermint.

*Nov 1, 2022*

## v1.0.0-rc0

### Bug Fixes

- [#748](https://github.com/crypto-org-chain/cronos/pull/748) Fix inconsistent state if upgrade migration commit is interrupted.
- [#752](https://github.com/crypto-org-chain/cronos/pull/752) Update iavl to `v0.19.4`.

*Oct 15, 2022*

## v0.9.0-beta4

### Bug Fixes

- [cronos#719](https://github.com/crypto-org-chain/cronos/pull/719) Fix `eth_call` for legacy blocks (backport #713).

### Improvements

- [cronos#720](https://github.com/crypto-org-chain/cronos/pull/720) Add option `iavl-disable-fastnode` to disable iavl fastnode indexing migration (backport #714).
- [cronos#721](https://github.com/crypto-org-chain/cronos/pull/721) Integrate the file state streamer (backport #702).
- [cronos#730](https://github.com/crypto-org-chain/cronos/pull/730) Update dependencies to recent versions (backport #729).

*Sep 20, 2022*

## v0.9.0-beta3

### Bug Fixes

- [cronos#696](https://github.com/crypto-org-chain/cronos/pull/696) Fix json-rpc apis for legacy blocks.

*Aug 29, 2022*

## v0.9.0-beta2

### State Machine Breaking
- [cronos#429](https://github.com/crypto-org-chain/cronos/pull/429) Update ethermint to main, ibc-go to v3.0.0, cosmos sdk to v0.45.4 and gravity to latest, remove v0.7.0 related upgradeHandler.
- [cronos#532](https://github.com/crypto-org-chain/cronos/pull/532) Add SendtoChain and CancelSendToChain support from evm call.
- [cronos#600](https://github.com/crypto-org-chain/cronos/pull/600) Implement bidirectional token mapping.
- [cronos#611](https://github.com/crypto-org-chain/cronos/pull/611) Fix mistake on acknowledgement error in ibc middleware.
- [cronos#627](https://github.com/crypto-org-chain/cronos/pull/627) Upgrade gravity bridge module with security enhancements
- [cronos#647](https://github.com/crypto-org-chain/cronos/pull/647) Integrate ibc fee middleware.
- [cronos#672](https://github.com/crypto-org-chain/cronos/pull/672) Revert interchain-accounts integration.

### Bug Fixes

- [cronos#502](https://github.com/crypto-org-chain/cronos/pull/502) Fix failed tx are ignored in json-rpc apis.
- [cronos#556](https://github.com/crypto-org-chain/cronos/pull/556) Bump gravity bridge module version to include bugfixes (including grpc endpoint)
- [cronos#639](https://github.com/crypto-org-chain/cronos/pull/639) init and validate-genesis commands don't include experimental modules by default.

### Improvements
- [cronos#418](https://github.com/crypto-org-chain/cronos/pull/418) Support logs in evm-hooks and return id for SendToEthereum events
- [cronos#489](https://github.com/crypto-org-chain/cronos/pull/489) Enable jemalloc memory allocator, and update rocksdb src to `v6.29.5`.
- [cronos#511](https://github.com/crypto-org-chain/cronos/pull/511) Replace ibc-hook with ibc middleware, use ibc-go upstream version.
- [cronos#550](https://github.com/crypto-org-chain/cronos/pull/550) Support basic json-rpc apis on pruned nodes.
- [cronos#549](https://github.com/crypto-org-chain/cronos/pull/549) Use custom tx indexer feature of ethermint.
- [cronos#673](https://github.com/crypto-org-chain/cronos/pull/673) Upgrade cosmos-sdk to 0.46.1 and ibc-go to v5.0.0-rc0.

*Aug 5, 2022*

## v0.8.0

### State Machine Breaking

- [cronos#618](https://github.com/crypto-org-chain/cronos/pull/618) selfdestruct don't delete bytecode of smart contract.

*Aug 5, 2022*

## v0.7.1

### Bug Fixes

- [cronos#454](https://github.com/crypto-org-chain/cronos/pull/454) Add back the latest testnet upgrade handler.
- [cronos#503](https://github.com/crypto-org-chain/cronos/pull/503) Fix failed tx are ignored in json-rpc apis (backport #502).
- [cronos#526](https://github.com/crypto-org-chain/cronos/pull/526) Fix tendermint duplicated tx issue.
- [cronos#584](https://github.com/crypto-org-chain/cronos/pull/584) Validate eth tx hash in ante handler and fix tx hashes returned in some JSON-RPC apis.
- [cronos#587](https://github.com/crypto-org-chain/cronos/pull/587) Unlucky tx patch cmd recompute eth tx hash.
- [cronos#595](https://github.com/crypto-org-chain/cronos/pull/595) Workaround the tx hash issue in event parsing.

### Improvements

- [cronos#489](https://github.com/crypto-org-chain/cronos/pull/489) Enable jemalloc memory allocator, and update rocksdb src to `v6.29.5`.
- [cronos#513](https://github.com/crypto-org-chain/cronos/pull/513) Add `fix-unlucky-tx` command to patch txs post v0.7.0 upgrade.
- [cronos#522](https://github.com/crypto-org-chain/cronos/pull/522) Add `reindex-duplicated-tx` command to handle the tendermint tx duplicated issue.
- [cronos#585](https://github.com/crypto-org-chain/cronos/pull/585) Reject replay unprotected tx, mainly the old transactions on ethereum.

*May 3, 2022*

## v0.7.0

### State Machine Breaking

- [cronos#241](https://github.com/crypto-org-chain/cronos/pull/241) Update ethermint to main and merged statedb refactoring in custom fork.
- [cronos#289](https://github.com/crypto-org-chain/cronos/pull/289) Update ethermint to `v0.10.0-cronos` which uses ibc-go `v2.0.2` instead of `v3.0.0-alpha2` and include the fixes below:
  - [ethermint#901](https://github.com/tharsis/ethermint/pull/901) support batch evm tx
  - [ethermint#849](https://github.com/tharsis/ethermint/pull/849) Change EVM hook interface.
  - [ethermint#809](https://github.com/tharsis/ethermint/pull/809) fix nonce increment issue when contract deployment tx get reverted.
  - [ethermint#855](https://github.com/tharsis/ethermint/pull/855) unify base fee related logic in the code.
  - [ethermint#817](https://github.com/tharsis/ethermint/pull/817) Fix eip-1559 logic related to effectiveGasPrice.
  - [ethermint#822](https://github.com/tharsis/ethermint/pull/822) Update base fee in begin blocker rather than end blocker.
  - [cosmos-sdk#10833](https://github.com/cosmos/cosmos-sdk/pull/10833) fix reported tx gas used when block gas limit exceeded.
  - [cosmos-sdk#10814](https://github.com/cosmos/cosmos-sdk/pull/10814) revert tx when block gas limit exceeded.
  - [cosmos-sdk#10725](https://github.com/cosmos/cosmos-sdk/pull/10725) populate `ctx.ConsensusParams` for begin/end blockers (fix baseFee calculation in ethermint).
- [cronos#315](https://github.com/crypto-org-chain/cronos/pull/315) Update cosmos-sdk to `v0.45.0`

### Improvements
- [cronos#210](https://github.com/crypto-org-chain/cronos/pull/210) re-enabling gravity bridge conditionally
- [cronos#322](https://github.com/crypto-org-chain/cronos/pull/322) Merge min-gas-price change in ethermint: don't check min-gas-price for EVM tx when feemarket enabled.
- [cronos#345](https://github.com/crypto-org-chain/cronos/pull/345) disable the url query parameter in swagger-ui.
- [cronos#328](https://github.com/crypto-org-chain/cronos/pull/328) display detail panic information in query result when `--trace` enabled.
- [cronos#441](https://github.com/crypto-org-chain/cronos/pull/441) Update cosmos-sdk to `v0.45.4`

### Bug Fixes
- [cronos#287](https://github.com/crypto-org-chain/cronos/pull/287) call upgrade handler before sealing app
- [cronos#323](https://github.com/crypto-org-chain/cronos/pull/323) Upgrade gravity bridge to v0.3.9 which contain a bugfix on `batchTxExecuted.`
- [cronos#324](https://github.com/crypto-org-chain/cronos/pull/324) Update to cosmos-sdk `v0.45.1`, which fixes an OOM issue.
- [cronos#329](https://github.com/crypto-org-chain/cronos/pull/329) Fix panic of eth_call on blocks prior to upgrade. 
- [cronos#340](https://github.com/crypto-org-chain/cronos/pull/340) Update dependencies to include several bug fixes: a) fix subscription deadlock issue in ethermint, b) fix data races `traceContext`.
- [cronos#370](https://github.com/crypto-org-chain/cronos/pull/370) Update ethermint to fix a websocket bug, add websockets integration tests.
- [cronos#378](https://github.com/crypto-org-chain/cronos/pull/378) Backport recent ethermint bug fixes: a) fix tx inclusion issue by report correct gasWanted of eth tx, b) Add buffer to eth_gasPrice response to fix client UX, c) Quick fix for eth_feeHistory when reward is nil, d) add returnValue message on tracing.
- [cronos#446](https://github.com/crypto-org-chain/cronos/pull/446) Fix failure of query legacy block after upgrade.

*December 10, 2021*

## v0.6.5

### Bug Fixes

- [cronos#255](https://github.com/crypto-org-chain/cronos/pull/255) fix empty topics in non-breaking way
- [cronos#270](https://github.com/crypto-org-chain/cronos/pull/270) reject MsgEthereumTx wrapping tx without the extension option.

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

This version contains several new features, it enables gravity bridge in Cronos and automatic token conversion for bridging tokens to crc20 tokens. It also fixes the decimal conversion issues in the CRO tokens from Crypto.org Chain.
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
