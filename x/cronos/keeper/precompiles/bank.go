package precompiles

import (
	"errors"
	"math/big"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/bank"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/evmos/ethermint/x/evm/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	EVMDenomPrefix      = "evm/"
	MintMethodName      = "mint"
	BurnMethodName      = "burn"
	BalanceOfMethodName = "balanceOf"
	TransferMethodName  = "transfer"
)

var (
	bankABI                 abi.ABI
	bankContractAddress     = common.BytesToAddress([]byte{100})
	bankGasRequiredByMethod = map[[4]byte]uint64{}
)

func init() {
	if err := bankABI.UnmarshalJSON([]byte(bank.BankModuleMetaData.ABI)); err != nil {
		panic(err)
	}
	for methodName := range bankABI.Methods {
		var methodID [4]byte
		copy(methodID[:], bankABI.Methods[methodName].ID[:4])
		switch methodName {
		case MintMethodName, BurnMethodName:
			bankGasRequiredByMethod[methodID] = 200000
		case BalanceOfMethodName:
			bankGasRequiredByMethod[methodID] = 10000
		case TransferMethodName:
			bankGasRequiredByMethod[methodID] = 150000
		default:
			bankGasRequiredByMethod[methodID] = 0
		}
	}
}

func EVMDenom(token common.Address) string {
	return EVMDenomPrefix + token.Hex()
}

type BankContract struct {
	bankKeeper  types.BankKeeper
	cdc         codec.Codec
	kvGasConfig storetypes.GasConfig
}

// NewBankContract creates the precompiled contract to manage native tokens
func NewBankContract(bankKeeper types.BankKeeper, cdc codec.Codec, kvGasConfig storetypes.GasConfig) vm.PrecompiledContract {
	return &BankContract{bankKeeper, cdc, kvGasConfig}
}

func (bc *BankContract) Address() common.Address {
	return bankContractAddress
}

// RequiredGas calculates the contract gas use
func (bc *BankContract) RequiredGas(input []byte) uint64 {
	// base cost to prevent large input size
	baseCost := uint64(len(input)) * bc.kvGasConfig.WriteCostPerByte
	var methodID [4]byte
	copy(methodID[:], input[:4])
	requiredGas, ok := bankGasRequiredByMethod[methodID]
	if ok {
		return requiredGas + baseCost
	}
	return baseCost
}

func (bc *BankContract) checkBlockedAddr(addr sdk.AccAddress) error {
	to, err := sdk.AccAddressFromBech32(addr.String())
	if err != nil {
		return err
	}
	if bc.bankKeeper.BlockedAddr(to) {
		return errorsmod.Wrapf(errortypes.ErrUnauthorized, "%s is not allowed to receive funds", to.String())
	}
	return nil
}

func (bc *BankContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	// parse input
	methodID := contract.Input[:4]
	method, err := bankABI.MethodById(methodID)
	if err != nil {
		return nil, err
	}
	stateDB := evm.StateDB.(ExtStateDB)
	precompileAddr := bc.Address()
	switch method.Name {
	case MintMethodName, BurnMethodName:
		if readonly {
			return nil, errors.New("the method is not readonly")
		}
		args, err := method.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		recipient := args[0].(common.Address)
		amount := args[1].(*big.Int)
		if amount.Sign() <= 0 {
			return nil, errors.New("invalid amount")
		}
		addr := sdk.AccAddress(recipient.Bytes())
		if err := bc.checkBlockedAddr(addr); err != nil {
			return nil, err
		}
		denom := EVMDenom(contract.Caller())
		amt := sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(amount))
		err = stateDB.ExecuteNativeAction(precompileAddr, nil, func(ctx sdk.Context) error {
			if err := bc.bankKeeper.IsSendEnabledCoins(ctx, amt); err != nil {
				return err
			}
			if method.Name == "mint" {
				if err := bc.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(amt)); err != nil {
					return errorsmod.Wrap(err, "fail to mint coins in precompiled contract")
				}
				if err := bc.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, sdk.NewCoins(amt)); err != nil {
					return errorsmod.Wrap(err, "fail to send mint coins to account")
				}
			} else {
				if err := bc.bankKeeper.SendCoinsFromAccountToModule(ctx, addr, types.ModuleName, sdk.NewCoins(amt)); err != nil {
					return errorsmod.Wrap(err, "fail to send burn coins to module")
				}
				if err := bc.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(amt)); err != nil {
					return errorsmod.Wrap(err, "fail to burn coins in precompiled contract")
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return method.Outputs.Pack(true)
	case BalanceOfMethodName:
		args, err := method.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		token := args[0].(common.Address)
		addr := args[1].(common.Address)
		// query from storage
		balance := bc.bankKeeper.GetBalance(stateDB.Context(), sdk.AccAddress(addr.Bytes()), EVMDenom(token)).Amount.BigInt()
		return method.Outputs.Pack(balance)
	case TransferMethodName:
		if readonly {
			return nil, errors.New("the method is not readonly")
		}
		args, err := method.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		sender := args[0].(common.Address)
		recipient := args[1].(common.Address)
		amount := args[2].(*big.Int)
		if amount.Sign() <= 0 {
			return nil, errors.New("invalid amount")
		}
		from := sdk.AccAddress(sender.Bytes())
		to := sdk.AccAddress(recipient.Bytes())
		if err := bc.checkBlockedAddr(to); err != nil {
			return nil, err
		}
		denom := EVMDenom(contract.Caller())
		amt := sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(amount))
		err = stateDB.ExecuteNativeAction(precompileAddr, nil, func(ctx sdk.Context) error {
			if err := bc.bankKeeper.IsSendEnabledCoins(ctx, amt); err != nil {
				return err
			}
			if err := bc.bankKeeper.SendCoins(ctx, from, to, sdk.NewCoins(amt)); err != nil {
				return errorsmod.Wrap(err, "fail to send coins in precompiled contract")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return method.Outputs.Pack(true)
	default:
		return nil, errors.New("unknown method")
	}
}
