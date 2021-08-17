# Cronos 0.5.1 Release Notes

This is the first release of the new scaffolded project where ethermint is imported as a library, it contains several compliance fixes on evm state machine and web3 rpc apis.

Instructions to upgrade testnet are going to be published later.

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
