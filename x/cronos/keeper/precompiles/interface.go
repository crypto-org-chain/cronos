package precompiles

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/evmos/ethermint/x/evm/statedb"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ExtStateDB defines extra methods of statedb to support stateful precompiled contracts
type ExtStateDB interface {
	vm.StateDB
	ExecuteNativeAction(contract common.Address, converter statedb.EventConverter, action func(ctx sdk.Context) error) error
	Context() sdk.Context
}
