package mempool

// EnablePreVerify wires the lock-free pre-verification hook for mempool.type=app.
// fn runs the stateless EVM signature check before the admission mutex; it must
// return nil for non-EVM txs (and on any signer-build failure) so they fall
// through to the fully-locked RunTx. Kept as an injected closure so this package
// stays decoupled from the EVM ante/keeper types.
//
// Wired after EvmKeeper construction (the hook needs it), unlike EnableRecheck
// whose deps exist during baseapp setup. Until called, InsertTxHandler stays
// fully locked.
func (a *Admitter) EnablePreVerify(fn func([]byte) error) {
	a.preVerify = fn
}
