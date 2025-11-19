package keeper

import (
	"fmt"
	"math/big"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/evmos/ethermint/x/evm/statedb"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGasCap defines the gas limit used to run internal evm call
const DefaultGasCap uint64 = 25000000

// CallEVM execute an evm message from native module
func (k Keeper) CallEVM(ctx sdk.Context, to *common.Address, data []byte, value *big.Int, gasLimit uint64) (*core.Message, *evmtypes.MsgEthereumTxResponse, error) {
	nonce := k.evmKeeper.GetNonce(ctx, types.EVMModuleAddress)
	msg := &core.Message{
		From:             types.EVMModuleAddress,
		To:               to,
		Nonce:            nonce,
		Value:            value, // amount
		GasLimit:         gasLimit,
		GasPrice:         big.NewInt(0),
		GasFeeCap:        nil,
		GasTipCap:        nil, // gasPrice
		Data:             data,
		AccessList:       nil, // accessList
		SkipNonceChecks:  false,
		SkipFromEOACheck: false,
	}
	ret, err := k.evmKeeper.ApplyMessage(ctx, msg, nil, true)
	if err != nil {
		return nil, nil, err
	}

	// if the call is from an precompiled contract call, then re-emit the logs into the original stateDB.
	if stateDB, ok := ctx.Value(statedb.StateDBContextKey).(vm.StateDB); ok {
		for _, l := range ret.Logs {
			stateDB.AddLog(l.ToEthereum())
		}
	}

	return msg, ret, nil
}

// CallModuleCRC21 call a method of ModuleCRC21 contract
func (k Keeper) CallModuleCRC21(ctx sdk.Context, contract common.Address, method string, args ...interface{}) ([]byte, error) {
	data, err := types.ModuleCRC21Contract.ABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	_, res, err := k.CallEVM(ctx, &contract, data, big.NewInt(0), DefaultGasCap)
	if err != nil {
		return nil, err
	}
	if res.Failed() {
		return nil, fmt.Errorf("call contract failed: %s, %s, %s", contract.Hex(), method, res.Ret)
	}
	return res.Ret, nil
}

// DeployModuleCRC21 deploy an embed crc21 contract
func (k Keeper) DeployModuleCRC21(ctx sdk.Context, denom string) (common.Address, error) {
	ctor, err := types.ModuleCRC21Contract.ABI.Pack("", denom, uint8(0), false)
	if err != nil {
		return common.Address{}, err
	}
	data := types.ModuleCRC21Contract.Bin
	data = append(data, ctor...)

	msg, res, err := k.CallEVM(ctx, nil, data, big.NewInt(0), DefaultGasCap)
	if err != nil {
		return common.Address{}, err
	}

	if res.Failed() {
		return common.Address{}, fmt.Errorf("contract deploy failed: %s", res.Ret)
	}
	return crypto.CreateAddress(types.EVMModuleAddress, msg.Nonce), nil
}

// ConvertCoinFromNativeToCRC21 convert native token to erc20 token
func (k Keeper) ConvertCoinFromNativeToCRC21(ctx sdk.Context, sender common.Address, coin sdk.Coin, autoDeploy bool) error {
	if !types.IsValidCoinDenom(coin.Denom) {
		return fmt.Errorf("coin %s is not supported for conversion", coin.Denom)
	}
	var err error
	// external contract is returned in preference to auto-deployed ones
	contract, found := k.GetContractByDenom(ctx, coin.Denom)
	if !found {
		if !autoDeploy {
			return fmt.Errorf("no contract found for the denom %s", coin.Denom)
		}
		contract, err = k.DeployModuleCRC21(ctx, coin.Denom)
		if err != nil {
			return err
		}
		k.SetAutoContractForDenom(ctx, coin.Denom, contract)

		k.Logger(ctx).Info(fmt.Sprintf("contract address %s created for coin denom %s", contract.String(), coin.Denom))
	}

	isSource := types.IsSourceCoin(coin.Denom)
	coins := sdk.NewCoins(coin)
	if isSource {
		// burn coins
		err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, sdk.AccAddress(sender.Bytes()), types.ModuleName, sdk.NewCoins(coin))
		if err != nil {
			return err
		}
		err = k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins)
		if err != nil {
			return err
		}
		// unlock crc tokens
		_, err = k.CallModuleCRC21(ctx, contract, "transfer_from_cronos_module", sender, coin.Amount.BigInt())
		if err != nil {
			return err
		}
	} else {
		// send coins to contract address
		err = k.bankKeeper.SendCoins(ctx, sdk.AccAddress(sender.Bytes()), sdk.AccAddress(contract.Bytes()), coins)
		if err != nil {
			return err
		}
		// mint crc tokens
		_, err = k.CallModuleCRC21(ctx, contract, "mint_by_cronos_module", sender, coin.Amount.BigInt())
		if err != nil {
			return err
		}
	}

	return nil
}

// ConvertCoinFromCRC21ToNative convert erc20 token to native token
func (k Keeper) ConvertCoinFromCRC21ToNative(ctx sdk.Context, contract, receiver common.Address, amount sdkmath.Int) error {
	denom, found := k.GetDenomByContract(ctx, contract)
	if !found {
		return fmt.Errorf("the contract address %s is not mapped to native token", contract.String())
	}

	isSource := types.IsSourceCoin(denom)
	coins := sdk.NewCoins(sdk.NewCoin(denom, amount))

	if isSource {
		_, err := k.CallModuleCRC21(ctx, contract, "transfer_by_cronos_module", receiver, amount.BigInt())
		if err != nil {
			return err
		}
		if err = k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
			return err
		}
		if err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sdk.AccAddress(receiver.Bytes()), coins); err != nil {
			return err
		}
	} else {
		err := k.bankKeeper.SendCoins(
			ctx,
			sdk.AccAddress(contract.Bytes()),
			sdk.AccAddress(receiver.Bytes()),
			coins,
		)
		if err != nil {
			return err
		}

		_, err = k.CallModuleCRC21(ctx, contract, "burn_by_cronos_module", receiver, amount.BigInt())
		if err != nil {
			return err
		}
	}

	return nil
}

// ConvertCoinsFromNativeToCRC21 convert native tokens to erc20 tokens
func (k Keeper) ConvertCoinsFromNativeToCRC21(ctx sdk.Context, sender common.Address, coins sdk.Coins, autoDeploy bool) error {
	for _, coin := range coins {
		if err := k.ConvertCoinFromNativeToCRC21(ctx, sender, coin, autoDeploy); err != nil {
			return err
		}
	}
	return nil
}
