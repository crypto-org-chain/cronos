package mempool

import (
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/ethermint/ante"
	"github.com/evmos/ethermint/ante/cache"
	ethermint "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PreVerifierRegistry collects lock-free admission pre-verifiers contributed by
// modules. The app composes them and hands Verify to the mempool manager, which
// runs it before the admission mutex (mempool.type=app). First rejection wins; a
// nil result defers to the locked admission path.
type PreVerifierRegistry struct {
	verifiers []func(sdk.Tx) error
}

// Register adds a pre-verifier; nil is ignored.
func (r *PreVerifierRegistry) Register(v func(sdk.Tx) error) {
	if v != nil {
		r.verifiers = append(r.verifiers, v)
	}
}

// Verify runs the registered pre-verifiers, returning the first rejection or nil.
func (r *PreVerifierRegistry) Verify(tx sdk.Tx) error {
	for _, v := range r.verifiers {
		if err := v(tx); err != nil {
			return err
		}
	}
	return nil
}

// NewEVMSigPreVerifier rejects pure-EVM txs with bad signatures and defers other txs to the locked path.
// It takes the tx admission already decoded, unlike ethermint's raw-bytes variant, so a tx is decoded once;
// keep its pure-EVM gate in sync with that variant.
func NewEVMSigPreVerifier(chainID string, senderCache *cache.SenderCache) func(sdk.Tx) error {
	cid, err := ethermint.ParseChainID(chainID)
	if err != nil {
		return nil
	}
	signer := ethtypes.LatestSignerForChainID(cid)

	return func(tx sdk.Tx) error {
		msgs := tx.GetMsgs()
		if len(msgs) == 0 {
			return nil
		}
		for _, msg := range msgs {
			if _, ok := msg.(*evmtypes.MsgEthereumTx); !ok {
				return nil
			}
		}
		return ante.VerifyEthSig(tx, signer, senderCache)
	}
}
