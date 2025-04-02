# Changelog

## UNRELEASED

### Bug Fixes

* [#1720](https://github.com/crypto-org-chain/cronos/pull/1720) Include the fix of performance regression after upgrade in iavl.
* [#1725](https://github.com/crypto-org-chain/cronos/pull/1725) Include the fix of deadlock when close tree in iavl.
* [#1724](https://github.com/crypto-org-chain/cronos/pull/1724) Include the fix of nonce management in batch tx in ethermint.
* [#1748](https://github.com/crypto-org-chain/cronos/pull/1748) Query with GetCFWithTS to compare both timestamp and key to avoid run fixdata multiple times.
* (versiondb) [#1751](https://github.com/crypto-org-chain/cronos/pull/1751) Add missing Destroy for read options to properly hold and release options reference.
* (versiondb) [#1758](https://github.com/crypto-org-chain/cronos/pull/1758) Avoid ReadOptions mutated by reference release in iterator.
* [#1759](https://github.com/crypto-org-chain/cronos/pull/1759) Fix version mismatch happen occasionally.
* [#1766](https://github.com/crypto-org-chain/cronos/pull/1766) Include a security patch from ibc-go `v8.7.0`.

### Improvements

* [#1747](https://github.com/crypto-org-chain/cronos/pull/1747) Skip batch initialization and flush when fixdata with dry-run.
* [#1779](https://github.com/crypto-org-chain/cronos/pull/1779) Upgrade rocksdb to `v9.11.2`.
* [#1780](https://github.com/crypto-org-chain/cronos/pull/1780) memiavl LoadVersion loads up to the target version instead of exact version.

### State Machine Breaking

* [#1722](https://github.com/crypto-org-chain/cronos/pull/1722) Include a security patch from cosmos sdk.

*Dec 9, 2024*

## v1.4.1

### Bug Fixes

* [#1714](https://github.com/crypto-org-chain/cronos/pull/1714) Avoid nil pointer error when query blocks before feemarket module gets enabled.
* [#1713](https://github.com/crypto-org-chain/cronos/pull/1713) Register legacy codec to allow query historical txs whose modules are removed (icaauth, authz).

### Improvements

* [#1712](https://github.com/crypto-org-chain/cronos/pull/1712) Upgrade rocksdb to `v9.8.4`.

*Dec 2, 2024*

## v1.4.0

### Improvements

* [#1705](https://github.com/crypto-org-chain/cronos/pull/1705)
  - Reproduce iavl prune bug in test
  - change iavl dependency back to upstream
  - fix prune command with async pruning

*Nov 26, 2024*

## v1.4.0-rc6

### Bug Fixes

* [#1702](https://github.com/crypto-org-chain/cronos/pull/1702) Update iavl to include prune fix.

### Improvements

* [#1701](https://github.com/crypto-org-chain/cronos/pull/1701) Update ibc-go to v8.5.2.

*Nov 18, 2024*

## v1.4.0-rc5

### State Machine Breaking

* [#1697](https://github.com/crypto-org-chain/cronos/pull/1697) Check max-tx-gas-wanted only in CheckTx mode.

*Nov 14, 2024*

## v1.4.0-rc4

### Bug Fixes

* [#1679](https://github.com/crypto-org-chain/cronos/pull/1679) Include no trace detail on insufficient balance fix.
* [#1685](https://github.com/crypto-org-chain/cronos/pull/1685) Add command to fix versiondb corrupted data.
* [#1690](https://github.com/crypto-org-chain/cronos/pull/1690) Include balance and gasPrice fix of debug trace api and fix nil pointer panic with legacy tx format.

### Improvements

* [#1684](https://github.com/crypto-org-chain/cronos/pull/1684) versiondb NewKVStore accept string as store name.
* [#1688](https://github.com/crypto-org-chain/cronos/pull/1688) Add Timestamp api to versiondb iterator.
* [#1692](https://github.com/crypto-org-chain/cronos/pull/1692) Set iavl async pruning option.

*Nov 6, 2024*

## v1.4.0-rc3

### Bug Fixes

* (iavl)[#1673](https://github.com/crypto-org-chain/cronos/pull/1673) Update iavl dependency to include pruning fix.

### Features

* [#1665](https://github.com/crypto-org-chain/cronos/pull/1665) Support register for payee and counterpartyPayee in relayer precompile.

### Improvements

* [#1664](https://github.com/crypto-org-chain/cronos/pull/1664) Update cometbft to 0.38.13.
* [#1660](https://github.com/crypto-org-chain/cronos/pull/1660) Support async check tx.
* [#1667](https://github.com/crypto-org-chain/cronos/pull/1667) Add testnet benchmark command.
* [#1669](https://github.com/crypto-org-chain/cronos/pull/1669) Add performance optimizations: a) async fireEvents, b) faster prepare proposal when using NopMempool, c) parallel check-tx
* [#1676](https://github.com/crypto-org-chain/cronos/pull/1676) Update cometbft to 0.38.14 and rocksdb to 9.7.4.

*Oct 24, 2024*

## v1.4.0-rc2

### Bug Fixes

* (testground)[1649](https://github.com/crypto-org-chain/cronos/pull/1649) Fix running single validator benchmark locally.
* (cli)[#1647](https://github.com/crypto-org-chain/cronos/pull/1647) Fix node can't shutdown by signal.
* (testground)[#1652](https://github.com/crypto-org-chain/cronos/pull/1652) Remove unexpected conflicts in benchmark transactions.
* [#1654](https://github.com/crypto-org-chain/cronos/pull/1654) Set relayer as payee for relayer caller when enabled incentivized packet.
* [#1655](https://github.com/crypto-org-chain/cronos/pull/1655) Fix state overwrite in debug trace APIs.
* [#1663](https://github.com/crypto-org-chain/cronos/pull/1663) Align attributes for ibc timeout event.

### Improvements

* [#1645](https://github.com/crypto-org-chain/cronos/pull/1645) Gen test tx in parallel even in single node.
* (testground)[#1644](https://github.com/crypto-org-chain/cronos/pull/1644) load generator retry with backoff on error.
* [#1648](https://github.com/crypto-org-chain/cronos/pull/1648) Add abort OE in PrepareProposal.
* (testground)[#1651](https://github.com/crypto-org-chain/cronos/pull/1651) Benchmark use cosmos broadcast rpc.
* (testground)[#1650](https://github.com/crypto-org-chain/cronos/pull/1650) Benchmark support batch mode.
* [#1658](https://github.com/crypto-org-chain/cronos/pull/1658) Optimize when block-list is empty.
* (testground)[#1659](https://github.com/crypto-org-chain/cronos/pull/1659) Support skip check-tx in benchmark.
* [#1662](https://github.com/crypto-org-chain/cronos/pull/1662) Emit more packet info for ibc relayer event.

*Oct 14, 2024*

## v1.4.0-rc1

### State Machine Breaking

* (memiavl)[#1618](https://github.com/crypto-org-chain/cronos/pull/1618) memiavl change initial version logic to be
  compatible with iavl 1.2.0.

### Improvements

* [#1592](https://github.com/crypto-org-chain/cronos/pull/1592) Change the default parallelism of the block-stm to minimum between GOMAXPROCS and NumCPU
* [#1600](https://github.com/crypto-org-chain/cronos/pull/1600) Update ethermint to avoid unnecessary block result in header related api call.
* [#1606](https://github.com/crypto-org-chain/cronos/pull/1606) Fix pebbledb support.
* [#1610](https://github.com/crypto-org-chain/cronos/pull/1610) Sync e2ee module with v1.3.x branch.
* [#1612](https://github.com/crypto-org-chain/cronos/pull/1612) Support ibc channel upgrade related methods.
* [#1614](https://github.com/crypto-org-chain/cronos/pull/1614) Bump cosmos-sdk to v0.50.10.
* [#1613](https://github.com/crypto-org-chain/cronos/pull/1613) Check admin sender for MsgStoreBlockList in check tx.

### Bug Fixes

* [#1609](https://github.com/crypto-org-chain/cronos/pull/1609) Fix query address-by-acc-num by account_id instead of id.
* [#1611](https://github.com/crypto-org-chain/cronos/pull/1611) Fix multisig account failed on threshold encode after send tx.
* [#1617](https://github.com/crypto-org-chain/cronos/pull/1617) Fix unsuppored sign mode SIGN_MODE_TEXTUAL for bank transfer.
* [#1621](https://github.com/crypto-org-chain/cronos/pull/1621), [1630](https://github.com/crypto-org-chain/cronos/pull/1630) Update ethermint to the fix of broken opBlockhash and tx validation.
* [#1623](https://github.com/crypto-org-chain/cronos/pull/1623) Ensure expedited related gov params pass the basic validation.
* [#1633](https://github.com/crypto-org-chain/cronos/pull/1633) Align acknowledgement with underlying_app_success when ack packet does not succeed.
* [#1638](https://github.com/crypto-org-chain/cronos/pull/1638) sync x/tx bug fixes.

*Sep 13, 2024*

## v1.4.0-rc0

### State Machine Breaking

* [#1377](https://github.com/crypto-org-chain/cronos/pull/1377) Upgrade sdk to 0.50, and integrate block-stm parallel tx execution.
* [#1394](https://github.com/crypto-org-chain/cronos/pull/1394) Add icahost wirings but disable in parameters.
* [#1414](https://github.com/crypto-org-chain/cronos/pull/1414) Integrate new evm tx format.
* [#1458](https://github.com/crypto-org-chain/cronos/pull/1458) Adjust require gas for recvPacket when ReceiverChainIsSource.
* [#1519](https://github.com/crypto-org-chain/cronos/pull/1519) Upgrade ibc-go to 8.3 and remove icaauth module.
* [#1518](https://github.com/crypto-org-chain/cronos/pull/1518) Keep versiondb/memiavl compatible with upstream sdk, stop supporting other streaming service.
* [#1541](https://github.com/crypto-org-chain/cronos/pull/1541) Disable MsgCreatePermanentLockedAccount and MsgCreatePeriodicVestingAccount messages.
* [#1552](https://github.com/crypto-org-chain/cronos/pull/1552) Avoid unnecessary GetAccount in ante handlers.

### Improvements

* (store) [#1378](https://github.com/crypto-org-chain/cronos/pull/1378) Upgrade rocksdb to `v8.11.3`.
* (versiondb) [#1387](https://github.com/crypto-org-chain/cronos/pull/1387) Add dedicated config section for versiondb, prepare for sdk 0.50 integration.
* (store) [#1448](https://github.com/crypto-org-chain/cronos/pull/1448) Upgrade rocksdb to `v9.1.1`.
* [#1431](https://github.com/crypto-org-chain/cronos/pull/1431) Integrate testground to run benchmark on cluster.
* [#1464](https://github.com/crypto-org-chain/cronos/pull/1464) Update cosmos-sdk to `v0.50.7`.
* [#1484](https://github.com/crypto-org-chain/cronos/pull/1484), [#1487](https://github.com/crypto-org-chain/cronos/pull/1487) Respect gas that is wanted to be returned by the ante handler.
* [#1488](https://github.com/crypto-org-chain/cronos/pull/1488) Enable optimistic execution.
* [#1490](https://github.com/crypto-org-chain/cronos/pull/1490) Update cometbft to v0.38.8.
* (versiondb) [#1491](https://github.com/crypto-org-chain/cronos/pull/1491) Free slice data in HasAtVersion.
* (versiondb) [#1498](https://github.com/crypto-org-chain/cronos/pull/1498) Reduce scope of copying slice data in iterator.
* [#1500](https://github.com/crypto-org-chain/cronos/pull/1500), [#1503](https://github.com/crypto-org-chain/cronos/pull/1503) Set mempool MaxTx from config (with a default value of `3000`).
* (store) [#1511](https://github.com/crypto-org-chain/cronos/pull/1511) Upgrade rocksdb to `v9.2.1`.
* (block-stm) [#1515](https://github.com/crypto-org-chain/cronos/pull/1515) Improve performance by cache signature verification result between incarnations of same tx.
* (store) [#1526](https://github.com/crypto-org-chain/cronos/pull/1526) Cache index/filters in rocksdb application.db to reduce ram usage.
* (store)[#1529](https://github.com/crypto-org-chain/cronos/pull/1529) Enable pinL0FilterAndIndexBlocksInCache.
* (store)[#1547](https://github.com/crypto-org-chain/cronos/pull/1547) Disable memiavl cache if block-stm is enabled.
* (app)[#1564](https://github.com/crypto-org-chain/cronos/pull/1564) Fix mempool data race.
* [#1568](https://github.com/crypto-org-chain/cronos/pull/1568) Update cometbft to 0.38.12.
* [#1570](https://github.com/crypto-org-chain/cronos/pull/1570) Integrate pre-estimate block-stm option to improve worst case performance.
* [#1572](https://github.com/crypto-org-chain/cronos/pull/1572) Allow disable sdk mempool by setting mempool.max-txs to `-1`.

### Bug Fixes

* [#1520](https://github.com/crypto-org-chain/cronos/pull/1520) Avoid invalid chain id for signer error when rpc call before chain id set in BeginBlock.
* [#1539](https://github.com/crypto-org-chain/cronos/pull/1539) Fix go-block-stm bug that causes app hash mismatch.
* [#1560](https://github.com/crypto-org-chain/cronos/pull/1560) Update queries contract addresses by native denom from a query in contract_by_denom.
* [#1569](https://github.com/crypto-org-chain/cronos/pull/1569) Update ethermint to fix of crash on chainID and mismatch tx hash in newHeads.

*Jun 18, 2024*

## v1.3.0-rc2

### Improvements

* (rpc) [#1467](https://github.com/crypto-org-chain/cronos/pull/1467) Avoid unnecessary tx decode in tx listener.

### Bug Fixes

* [#1466](https://github.com/crypto-org-chain/cronos/pull/1466) Fix handling of pending transactions related APIs.

*May 21, 2024*

## v1.3.0-rc1

### State Machine Breaking

* [#1407](https://github.com/crypto-org-chain/cronos/pull/1407) Add end-to-end encryption module.

### Improvements

* [#1413](https://github.com/crypto-org-chain/cronos/pull/1413) Add custom keyring implementation for e2ee module.
* (e2ee)[#1415](https://github.com/crypto-org-chain/cronos/pull/1415) Add batch keys query for e2ee module.
* (e2ee)[#1421](https://github.com/crypto-org-chain/cronos/pull/1421) Validate e2ee key when register.
* [#1437](https://github.com/crypto-org-chain/cronos/pull/1437) Update cometbft and cosmos-sdk dependencies.

### Bug Fixes

* (rpc) [#1444](https://github.com/crypto-org-chain/cronos/pull/1444) Avoid nil pointer error when query blocks before feemarket module gets enabled.
* [#1439](https://github.com/crypto-org-chain/cronos/pull/1439) Add back default prepare proposal logic.

*May 3, 2024*

## v1.2.2

### Bug Fixes

* (rpc) [#1416](https://github.com/crypto-org-chain/cronos/pull/1416) Fix parsed logs from old events.

*April 22, 2024*

## v1.2.1

### Improvements

* (test) [#1380](https://github.com/crypto-org-chain/cronos/pull/1380) Upgrade cosmovisor to 1.5.0 in integration test.
* (versiondb) [#1379](https://github.com/crypto-org-chain/cronos/pull/1379) Flush versiondb when graceful shutdown, make rocksdb upgrade smooth.

### Bug Fixes

* (rpc) [#1397](https://github.com/crypto-org-chain/cronos/pull/1397) Avoid panic on invalid elasticity_multiplier.

### Features

* [#1406](https://github.com/crypto-org-chain/cronos/pull/1406) Add set-encryption-key for encryption module.
* [#1411](https://github.com/crypto-org-chain/cronos/pull/1411) Add encrypt and decrypt cmds for message.

*April 8, 2024*

## v1.2.0
## v1.2.0-rc1

### Bug Fixes

* (rpc) [#1371](https://github.com/crypto-org-chain/cronos/pull/1371) Add param keytable in evm for old upgrade.

*April 2, 2024*

## v1.2.0-rc0

### Bug Fixes

- [#1363](https://github.com/crypto-org-chain/cronos/pull/1363) Update ethermint to fix a panic on overflow and patch gasUsed in the RPC API.

### State Machine Breaking

* [#1366](https://github.com/crypto-org-chain/ethermint/pull/1366) Keep behavior of random opcode as before.

*March 26, 2024*

## v1.1.1

### Improvements

- [#1362](https://github.com/crypto-org-chain/cronos/pull/1362) Log blacklist addresses.

*March 19, 2024*

## v1.1.0

### Bug Fixes

- [#1336](https://github.com/crypto-org-chain/cronos/pull/1336) Update ethermint to develop to fix feeHistory rpc api.

*February 28, 2024*

## v1.1.0-rc5

### Bug Fixes

- [#1329](https://github.com/crypto-org-chain/cronos/pull/1329) Update cosmos-sdk to `v0.47.10`.

*February 19, 2024*

## v1.1.0-rc4

### State Machine Breaking

- [#1318](https://github.com/crypto-org-chain/cronos/pull/1318) Add packet_sequence index in relayer event.
- [#1318](https://github.com/crypto-org-chain/cronos/pull/1318) Fix filter rule for eth_getLogs.
- [#1322](https://github.com/crypto-org-chain/cronos/pull/1322) Add `v1.1.0-testnet-1` upgrade plan for testnet.

### Improvements

- [#1324](https://github.com/crypto-org-chain/cronos/pull/1324) Update cosmos-sdk to `v0.47.9`.

*February 5, 2024*

## v1.1.0-rc3

### Bug Fixes

- [#1292](https://github.com/crypto-org-chain/cronos/pull/1292) memiavl cancel background snapshot rewriting when graceful shutdown.
- [#1294](https://github.com/crypto-org-chain/cronos/pull/1294) Update ethermint to fix and improve of debug_traceCall and eth_feeHistory.
- [#1302](https://github.com/crypto-org-chain/cronos/pull/1302) Fix concurrent map access in rootmulti store.
- [#1304](https://github.com/crypto-org-chain/cronos/pull/1304) Write versiondb with fsync, and relax the version requirement on startup.
- [#1308](https://github.com/crypto-org-chain/cronos/pull/1308) Update ethermint to fix duplicate cache events emitted from evm hooks and wrong priority tx.
- [#1311](https://github.com/crypto-org-chain/cronos/pull/1311) Add missing version in memiavl log.

### Improvements

- [#1291](https://github.com/crypto-org-chain/cronos/pull/1291) Update ibc-go to v7.3.2.
- [#1309](https://github.com/crypto-org-chain/cronos/pull/1309) Add missing destroy for file lock and close map on error.

*January 5, 2024*

## v1.1.0-rc2

- [#1258](https://github.com/crypto-org-chain/cronos/pull/1258) Support hard-fork style upgrades.
- [#1272](https://github.com/crypto-org-chain/cronos/pull/1272) Update ethermint to develop, cosmos-sdk to `v0.47.7`.
- [#1273](https://github.com/crypto-org-chain/cronos/pull/1273) Enable push0 opcode in integration test.
- [#1274](https://github.com/crypto-org-chain/cronos/pull/1274) Remove authz module.
- [#1287](https://github.com/crypto-org-chain/cronos/pull/1287) Support debug_traceCall.

### Bug Fixes

- [#1215](https://github.com/crypto-org-chain/cronos/pull/1215) Update ethermint to fix of concurrent write in fee history.
- [#1217](https://github.com/crypto-org-chain/cronos/pull/1217) Use the default chain-id behavour in sdk.
- [#1216](https://github.com/crypto-org-chain/cronos/pull/1216) Update ethermint to fix of avoid redundant parse chainID from gensis when start server.
- [#1230](https://github.com/crypto-org-chain/cronos/pull/1230) Fix mem store in versiondb multistore.
- [#1233](https://github.com/crypto-org-chain/cronos/pull/1233) Re-emit logs in callback contract.
- [#1256](https://github.com/crypto-org-chain/cronos/pull/1256) Improve permission checkings for some messages.

### State Machine Breaking

- [#1232](https://github.com/crypto-org-chain/cronos/pull/1232) Adjust require gas in relayer precompile to be closed with actual consumed.
- [#1209](https://github.com/crypto-org-chain/cronos/pull/1209) Support accurate estimate gas in evm tx from relayer.
- [#1247](https://github.com/crypto-org-chain/cronos/pull/1247) Update ethermint to develop, go-ethereum to `v1.11.2`.
- [#1235](https://github.com/crypto-org-chain/cronos/pull/1235) Add channel detail in ica packet callback.
- [#1251](https://github.com/crypto-org-chain/cronos/pull/1251) Adjust require gas for submitMsgs in ica precompile.
- [#1252](https://github.com/crypto-org-chain/cronos/pull/1252) Add plan `v1.1.0-testnet` to update default max_callback_gas param.

### Improvements

- [#1239](https://github.com/crypto-org-chain/cronos/pull/1239) Refactor websocket/subscription system to improve performance and stability.
- [#1241](https://github.com/crypto-org-chain/cronos/pull/1241) Improve parallelization of memiavl restoration.
- (deps) [#1253](https://github.com/crypto-org-chain/cronos/pull/1253) Upgrade Go-Ethereum version to [`v1.11.6`](https://github.com/ethereum/go-ethereum/releases/tag/v1.11.6).

*October 17, 2023*

## v1.1.0-rc1

### Bug Fixes

- [#1206](https://github.com/crypto-org-chain/cronos/pull/1206) Add missing keypair of SendEnabled to restore legacy param set before migration.
- [#1205](https://github.com/crypto-org-chain/cronos/pull/1205) Fix versiondb and memiavl upgrade issues, add integration test.

### Improvements

- [#1197](https://github.com/crypto-org-chain/cronos/pull/1197) tune rocksdb options to control memory consumption.

*October 9, 2023*

## v1.1.0-rc0

### State Machine Breaking

- [cronos#695](https://github.com/crypto-org-chain/cronos/pull/695) Implement ADR-007, generic events format with indexed params.
- [cronos#728](https://github.com/crypto-org-chain/cronos/pull/728) Upgrade gravity bridge latest bugfix, including multi attestation processing and double spend check.
- [cronos#742](https://github.com/crypto-org-chain/cronos/pull/742) Add upgrade handler for v0.8.0-gravity-alpha2.
- [cronos#750](https://github.com/crypto-org-chain/cronos/pull/750) Add upgrade handler for v0.8.0-gravity-alpha3.
- [cronos#769](https://github.com/crypto-org-chain/cronos/pull/769) Prevent cancellation function to be called outside the scope of the contract that manage it.
- [cronos#775](https://github.com/crypto-org-chain/cronos/pull/775) Support turnbridge transaction.
- [cronos#781](https://github.com/crypto-org-chain/cronos/pull/781) Add prune command.
- [cronos#830](https://github.com/crypto-org-chain/cronos/pull/830) Upgrade gravity bridge for latest bugfixes, patching two important DOS vulnerabilities
- [cronos#834](https://github.com/crypto-org-chain/cronos/pull/834) Remove unsafe experimental flag.
- [cronos#842](https://github.com/crypto-org-chain/cronos/pull/842) Add upgrade handler for v2.0.0-testnet3.
- [cronos#795](https://github.com/crypto-org-chain/cronos/pull/795) Support permissions in cronos.
- [cronos#997](https://github.com/crypto-org-chain/cronos/pull/997) Fix logic to support proxy contract for cronos originated crc20.
- [cronos#1005](https://github.com/crypto-org-chain/cronos/pull/1005) Support specify channel id for send-to-ibc event in case of source token.
- [cronos#1069](https://github.com/crypto-org-chain/cronos/pull/1069) Update ethermint to develop, go-ethereum to `v1.10.26` and ibc-go to `v6.2.0`.
- [cronos#1147](https://github.com/crypto-org-chain/cronos/pull/1147) Integrate ica module.
- (deps) [#1121](https://github.com/crypto-org-chain/cronos/pull/1121) Bump Cosmos-SDK to v0.47.5 and ibc-go to v7.2.0.
- [cronos#1014](https://github.com/crypto-org-chain/cronos/pull/1014) Support stateful precompiled contract for relayer.
- [cronos#1165](https://github.com/crypto-org-chain/cronos/pull/1165) Icaauth module is not adjusted correctly in ibc-go v7.2.0.
- [cronos#1163](https://github.com/crypto-org-chain/cronos/pull/1163) Support stateful precompiled contract for ica.
- [cronos#837](https://github.com/crypto-org-chain/cronos/pull/837) Support stateful precompiled contract for bank.
- [cronos#1184](https://github.com/crypto-org-chain/cronos/pull/1184) Update ibc-go to `v7.3.1`.
- [cronos#1186](https://github.com/crypto-org-chain/cronos/pull/1186) Enlarge the max block gas limit in new version.
- [cronos#1187](https://github.com/crypto-org-chain/cronos/pull/1187) Disable gravity module in app.
- [cronos#1185](https://github.com/crypto-org-chain/cronos/pull/1185) Support ibc callback.
- [cronos#1196](https://github.com/crypto-org-chain/cronos/pull/1196) Skip register stateful precompiled contract for bank.

### Bug Fixes

- [#833](https://github.com/crypto-org-chain/cronos/pull/833) Fix rollback command.
- [#945](https://github.com/crypto-org-chain/cronos/pull/945) Fix no handler exists for proposal type error when update-client due to wrong ibc route.
- [#1036](https://github.com/crypto-org-chain/cronos/pull/1036) Fix memiavl import memory leak.
- [#1038](https://github.com/crypto-org-chain/cronos/pull/1038) Update ibc-go to `v5.2.1`.
- [#1042](https://github.com/crypto-org-chain/cronos/pull/1042) Avoid channel get changed when concurrent subscribe happens ([ethermint commit](https://github.com/crypto-org-chain/ethermint/commit/72bbe0a80dfd3c586868e2f0b4fbed72593c45bf)).
- [#1058](https://github.com/crypto-org-chain/cronos/pull/1058) Fix decode log for multi topics in websocket subscribe ([ethermint commit](https://github.com/crypto-org-chain/ethermint/commit/2136ad029860c819942ad1836dd3f42585002233)).
- [#1062](https://github.com/crypto-org-chain/cronos/pull/1062) Update cometbft `v0.34.29` with several minor bug fixes and low-severity security-fixes.
- [#1075](https://github.com/crypto-org-chain/cronos/pull/1075) Add missing close in memiavl to avoid resource leaks.
- [#1073](https://github.com/crypto-org-chain/cronos/pull/1073) memiavl automatically truncate corrupted wal tail.
- [#1087](https://github.com/crypto-org-chain/cronos/pull/1087) memiavl fix LastCommitID when memiavl db not loaded.
- [#1088](https://github.com/crypto-org-chain/cronos/pull/1088) memiavl fix empty value in write-ahead-log replaying.
- [#1102](https://github.com/crypto-org-chain/cronos/pull/1102) avoid duplicate cache events emitted from ibc and gravity hook.
- [#1123](https://github.com/crypto-org-chain/cronos/pull/1123) Fix memiavl snapshot switching
- [#1125](https://github.com/crypto-org-chain/cronos/pull/1125) Fix genesis migrate for feeibc, evm, feemarket and gravity.
- [#1130](https://github.com/crypto-org-chain/cronos/pull/1130) Fix lock issues when state-sync with memiavl.
- [#1150](https://github.com/crypto-org-chain/cronos/pull/1150) Fix memiavl's unsafe retain of the root hashes.

### Features

- [#1042](https://github.com/crypto-org-chain/cronos/pull/1042) call Close method on app to cleanup resource on graceful shutdown ([ethermint commit](https://github.com/crypto-org-chain/ethermint/commit/0ea7b86532a1144f229961f94b4524d5889e874d)).
- [#1083](https://github.com/crypto-org-chain/cronos/pull/1083) memiavl support both sdk 46 and 47 root hash rules.
- [#1091](https://github.com/crypto-org-chain/cronos/pull/1091) memiavl support rollback.
- [#1100](https://github.com/crypto-org-chain/cronos/pull/1100) memiavl support read-only mode, and grab exclusive lock for write mode.
- [#1103](https://github.com/crypto-org-chain/cronos/pull/1103) Add EventQueryTxFor cmd to subscribe and wait for transaction.
- [#1108](https://github.com/crypto-org-chain/cronos/pull/1108) versiondb support restore from local snapshot.
- [#1114](https://github.com/crypto-org-chain/cronos/pull/1114) memiavl support `CacheMultiStoreWithVersion`.
- [#1116](https://github.com/crypto-org-chain/cronos/pull/1116) versiondb commands support sdk47 app hash calculation.

### Improvements

- [#890](https://github.com/crypto-org-chain/cronos/pull/890) optimize memiavl snapshot format.
- [#904](https://github.com/crypto-org-chain/cronos/pull/904) Enable "dynamic-level-bytes" on new `application.db`.
- [#924](https://github.com/crypto-org-chain/cronos/pull/924) memiavl support `Export` API.
- [#950](https://github.com/crypto-org-chain/cronos/pull/950) Implement memiavl and integrate with state machine.
- [#985](https://github.com/crypto-org-chain/cronos/pull/985) Fix versiondb verify command on older versions
- [#998](https://github.com/crypto-org-chain/cronos/pull/998) Bump grocksdb to v1.7.16 and rocksdb to v7.10.2
- [#1028](https://github.com/crypto-org-chain/cronos/pull/1028) Add memiavl configs into app.toml
- [#1027](https://github.com/crypto-org-chain/cronos/pull/1027) Integrate local state-sync commands.
- [#1029](https://github.com/crypto-org-chain/cronos/pull/1029) Change config `async-commit` to `async-commit-buffer`, make the channel size configurable.
- [#1034](https://github.com/crypto-org-chain/cronos/pull/1034) Support memiavl snapshot strategy configuration.
- [#1035](https://github.com/crypto-org-chain/cronos/pull/1035) Support caching in memiavl directly, ignore inter-block cache silently.
- [#1050](https://github.com/crypto-org-chain/cronos/pull/1050) nativebyteorder mode will check endianness on startup, binaries are built with nativebyteorder by default.
- [#1064](https://github.com/crypto-org-chain/cronos/pull/1064) Simplify memiavl snapshot switching.
- [#1067](https://github.com/crypto-org-chain/cronos/pull/1067) memiavl: only export state-sync snapshots on an exist snapshot
- [#1082](https://github.com/crypto-org-chain/cronos/pull/1082) Make memiavl setup code reusable.
- [#1092](https://github.com/crypto-org-chain/cronos/pull/1092) memiavl disable sdk address cache if zero-copy enabled, and disable zero-copy by default.
- [#1099](https://github.com/crypto-org-chain/cronos/pull/1099) clean up memiavl tmp directories left behind.
- [#940](https://github.com/crypto-org-chain/cronos/pull/940) Update rocksdb dependency to 8.1.1.
- [#1149](https://github.com/crypto-org-chain/cronos/pull/1149) memiavl support `WorkingHash` api required by `FinalizeBlock`.
- [#1151](https://github.com/crypto-org-chain/cronos/pull/1151) memiavl `CacheMultiStoreWithVersion` supports `io.Closer`.
- [#1154](https://github.com/crypto-org-chain/cronos/pull/1154) Remove dependency on cosmos-sdk.
- [#1171](https://github.com/crypto-org-chain/cronos/pull/1171) Add memiavl background snapshot writing concurrency limit.
- [#1179](https://github.com/crypto-org-chain/cronos/pull/1179) Support blocking addresses in mempool.
- [#1182](https://github.com/crypto-org-chain/cronos/pull/1182) Bump librocksdb to 8.5.3.
- [#1183](https://github.com/crypto-org-chain/cronos/pull/1183) Avoid redundant logs added from relayer.

*April 13, 2023*

## v1.0.7

### Improvements

- [#936](https://github.com/crypto-org-chain/cronos/pull/936) Reuse recovered sender address to optimize performance ([ethermint commit](https://github.com/crypto-org-chain/ethermint/commit/cb741e1d819683795aa32e286d31d8155f903cae)).
- [#949](https://github.com/crypto-org-chain/cronos/pull/949) Release static-linked binaries for linux platform.
- [#934](https://github.com/crypto-org-chain/cronos/pull/934) Add pebbledb backend.

### Bug Fixes

- [#953](https://github.com/crypto-org-chain/cronos/pull/953) Include third-party bug fixes:
  - update ethermint to include two bug fixes
    - <https://github.com/crypto-org-chain/ethermint/pull/234>
    - <https://github.com/crypto-org-chain/ethermint/pull/233>
  - update cosmos-sdk to include one bug fix
    - <https://github.com/cosmos/cosmos-sdk/pull/15667>
- [#945](https://github.com/crypto-org-chain/cronos/pull/945) Fix no handler exists for proposal type error when update-client due to wrong ibc route.

*Mar 16, 2023*

## v1.0.6

### Bug Fixes

- [#932](https://github.com/crypto-org-chain/cronos/pull/932) Backport multiple json-rpc bug fixes in ethermint ([commits](https://github.com/crypto-org-chain/ethermint/compare/v0.20.8-cronos...v0.20.9-cronos)).

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

### Improvements

- [#813](https://github.com/crypto-org-chain/cronos/pull/813) Tune up rocksdb options.
- [#791](https://github.com/crypto-org-chain/cronos/pull/791) Implement versiondb and migration commands.
- [#779](https://github.com/crypto-org-chain/cronos/pull/779) Add config iavl-lazy-loading to enable lazy loading of iavl store.

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

*September 13, 2022*

## v0.9.0

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
