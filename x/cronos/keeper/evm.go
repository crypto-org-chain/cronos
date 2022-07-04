package keeper

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

// DefaultGasCap defines the gas limit used to run internal evm call
const DefaultGasCap uint64 = 25000000

// CallEVM execute an evm message from native module
func (k Keeper) CallEVM(ctx sdk.Context, to *common.Address, data []byte, value *big.Int) (*ethtypes.Message, *evmtypes.MsgEthereumTxResponse, error) {
	nonce := k.evmKeeper.GetNonce(ctx, types.EVMModuleAddress)
	msg := ethtypes.NewMessage(
		types.EVMModuleAddress,
		to,
		nonce,
		value, // amount
		DefaultGasCap,
		big.NewInt(0), nil, nil, // gasPrice
		data,
		nil,   // accessList
		false, // isFake
	)

	ret, err := k.evmKeeper.ApplyMessage(ctx, msg, nil, true)
	if err != nil {
		return nil, nil, err
	}
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

// DeployModuleCRC21 deploy an embed crc21 contract
func (k Keeper) DeployModuleCRC21(ctx sdk.Context, denom string) (common.Address, error) {
	ctor, err := types.ModuleCRC21Contract.ABI.Pack("", denom, uint8(0))
	if err != nil {
		return common.Address{}, err
	}
	data := types.ModuleCRC21Contract.Bin
	data = append(data, ctor...)

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
		return fmt.Errorf("coin %s is not supported for wrapping", coin.Denom)
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
	err = k.bankKeeper.SendCoins(ctx, sdk.AccAddress(sender.Bytes()), sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
	if err != nil {
		return err
	}
	_, err = k.CallModuleCRC20(ctx, contract, "mint_by_cronos_module", sender, coin.Amount.BigInt())
	if err != nil {
		return err
	}

	return nil
}

// ConvertCoinFromCRC20ToNative convert erc20 token to native token
func (k Keeper) ConvertCoinFromCRC20ToNative(ctx sdk.Context, contract common.Address, receiver common.Address, amount sdk.Int) error {
	denom, found := k.GetDenomByContract(ctx, contract)
	if !found {
		return fmt.Errorf("the contract address %s is not mapped to native token", contract.String())
	}

	err := k.bankKeeper.SendCoins(
		ctx,
		sdk.AccAddress(contract.Bytes()),
		sdk.AccAddress(receiver.Bytes()),
		sdk.NewCoins(sdk.NewCoin(denom, amount)),
	)
	if err != nil {
		return err
	}

	_, err = k.CallModuleCRC20(ctx, contract, "burn_by_cronos_module", receiver, amount.BigInt())
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
