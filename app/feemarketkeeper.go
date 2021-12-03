package app

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"
	feemarkettypes "github.com/tharsis/ethermint/x/feemarket/types"
)

const DefaultGasPrice int64 = 5000000000000

var _ evmtypes.FeeMarketKeeper = &ConstantFeeMarketKeeper{}

// ConstantFeeMarketKeeper implements a fee market keeper that returns a constant base fee
type ConstantFeeMarketKeeper struct{}

func (fmk ConstantFeeMarketKeeper) GetBaseFee(ctx sdk.Context) *big.Int {
	return big.NewInt(DefaultGasPrice)
}

func (fmk ConstantFeeMarketKeeper) GetParams(ctx sdk.Context) feemarkettypes.Params {
	return feemarkettypes.Params{
		NoBaseFee:                false,
		BaseFeeChangeDenominator: 8,
		ElasticityMultiplier:     2,
		InitialBaseFee:           DefaultGasPrice,
		EnableHeight:             0,
	}
}
