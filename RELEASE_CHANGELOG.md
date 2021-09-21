# Cronos 0.5.3 Release Notes

This version contains several new features, it enables gravity bridge in Cronos and automatic token conversion for bridging tokens to crc20 tokens. It also fix the decimal conversion issues in the CRO tokens from crypto.org chain.
In addition to that, it also upgrade ethermint to its latest version (v0.5.0.x) which bring several breaking changes (see [changelog](https://github.com/tharsis/ethermint/blob/1a01c6a992c0fb94d70bb1c7127715874cefd057/CHANGELOG.md))

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