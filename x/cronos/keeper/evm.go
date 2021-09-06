package keeper

import (
	"errors"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tharsis/ethermint/server/config"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

// CallEVM execute an evm message from native module
func (k Keeper) CallEVM(ctx sdk.Context, to *common.Address, data []byte, value *big.Int) (*ethtypes.Message, *evmtypes.MsgEthereumTxResponse, error) {
	k.evmKeeper.WithContext(ctx)

	nonce := k.evmKeeper.GetNonce(types.EVMModuleAddress)
	msg := ethtypes.NewMessage(
		types.EVMModuleAddress,
		to,
		nonce,
		value, // amount
		config.DefaultGasCap,
		big.NewInt(0), // gasPrice
		data,
		nil,   // accessList
		false, // checkNonce
	)

	params := k.evmKeeper.GetParams(ctx)
	// return error if contract creation or call are disabled through governance
	if !params.EnableCreate && to == nil {
		return nil, nil, errors.New("failed to create new contract")
	} else if !params.EnableCall && to != nil {
		return nil, nil, errors.New("failed to call contract")
	}
	ethCfg := params.ChainConfig.EthereumConfig(k.evmKeeper.ChainID())

	// get the coinbase address from the block proposer
	coinbase, err := k.evmKeeper.GetCoinbaseAddress(ctx)
	if err != nil {
		return nil, nil, errors.New("failed to obtain coinbase address")
	}
	evm := k.evmKeeper.NewEVM(msg, ethCfg, params, coinbase, nil)
	ret, err := k.evmKeeper.ApplyMessage(evm, msg, ethCfg, true)
	if err != nil {
		return nil, nil, err
	}
	k.evmKeeper.CommitCachedContexts()
	return &msg, ret, nil
}

// CallModuleCRC20 call a method of ModuleCRC20 contract
func (k Keeper) CallModuleCRC20(ctx sdk.Context, contract common.Address, method string, args ...interface{}) ([]byte, error) {
	data, err := types.ModuleCRC20Contract.ABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	_, res, err := k.CallEVM(ctx, &contract, data, big.NewInt(0))
	if err != nil {
		return nil, err
	}
	if res.Failed() {
		return nil, fmt.Errorf("call contract failed: %s, %s, %s", contract.Hex(), method, res.Ret)
	}
	return res.Ret, nil
}

// DeployModuleCRC20 deploy an embed erc20 contract
func (k Keeper) DeployModuleCRC20(ctx sdk.Context, denom string) (common.Address, error) {
	ctor, err := types.ModuleCRC20Contract.ABI.Pack("", denom, uint8(0))
	if err != nil {
		return common.Address{}, err
	}
	data := append(types.ModuleCRC20Contract.Bin, ctor...)

	msg, res, err := k.CallEVM(ctx, nil, data, big.NewInt(0))
	if err != nil {
		return common.Address{}, err
	}

	if res.Failed() {
		return common.Address{}, fmt.Errorf("contract deploy failed: %s", res.Ret)
	}
	return crypto.CreateAddress(types.EVMModuleAddress, msg.Nonce()), nil
}

// ConvertCoinFromNativeToCRC20 convert native token to erc20 token
func (k Keeper) ConvertCoinFromNativeToCRC20(ctx sdk.Context, sender common.Address, coin sdk.Coin, autoDeploy bool) error {
	if !types.IsValidDenomToWrap(coin.Denom) {
		return errors.New("denom is not supported for wrapping")
	}

	var err error
	// external contract is returned in preference to auto-deployed ones
	contract, found := k.GetContractByDenom(ctx, coin.Denom)
	if !found {
		if !autoDeploy {
			return errors.New("no contract found for the denom")
		}
		contract, err = k.DeployModuleCRC20(ctx, coin.Denom)
		if err != nil {
			return err
		}
		k.SetAutoContractForDenom(ctx, coin.Denom, contract)
	}
	err = k.bankKeeper.SendCoins(ctx, sdk.AccAddress(sender.Bytes()), sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
	if err != nil {
		return err
	}
	_, err = k.CallModuleCRC20(ctx, contract, "mint_by_native", sender, coin.Amount.BigInt())
	if err != nil {
		return err
	}

	return nil
}

// ConvertCoinFromCRC20ToNative convert erc20 token to native token
func (k Keeper) ConvertCoinFromCRC20ToNative(ctx sdk.Context, contract common.Address, receiver common.Address, coin sdk.Coin) error {
	if !types.IsValidDenomToWrap(coin.Denom) {
		return errors.New("denom is not supported for wrapping")
	}

	// validate contract address
	externalContract, externalFound := k.getExternalContractByDenom(ctx, coin.Denom)
	autoContract, autoFound := k.getAutoContractByDenom(ctx, coin.Denom)
	found := (externalFound && externalContract == contract) || (autoFound && autoContract == contract)
	if !found {
		return errors.New("the contract address is not mapped to the denom")
	}

	err := k.bankKeeper.SendCoins(ctx, sdk.AccAddress(contract.Bytes()), sdk.AccAddress(receiver.Bytes()), sdk.NewCoins(coin))
	if err != nil {
		return err
	}

	_, err = k.CallModuleCRC20(ctx, contract, "burn_by_native", receiver, coin.Amount.BigInt())
	if err != nil {
		return err
	}

	return nil
}

// ConvertCoinsFromNativeToCRC20 convert native tokens to erc20 tokens
func (k Keeper) ConvertCoinsFromNativeToCRC20(ctx sdk.Context, sender common.Address, coins sdk.Coins, autoDeploy bool) error {
	for _, coin := range coins {
		if err := k.ConvertCoinFromNativeToCRC20(ctx, sender, coin, autoDeploy); err != nil {
			return err
		}
	}
	return nil
}
