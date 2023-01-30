package keeper

import (
	"fmt"
	"strings"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	// this line is used by starport scaffolding # ibc/keeper/import
)

type (
	Keeper struct {
		cdc      codec.Codec
		storeKey storetypes.StoreKey
		memKey   storetypes.StoreKey

		// update balance and accounting operations with coins
		bankKeeper types.BankKeeper
		// ibc transfer operations
		transferKeeper types.TransferKeeper
		// gravity bridge keeper
		gravityKeeper types.GravityKeeper
		// ethermint evm keeper
		evmKeeper types.EvmKeeper
		// account keeper
		accountKeeper types.AccountKeeper

		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string

		// this line is used by starport scaffolding # ibc/keeper/attribute
	}
)

func NewKeeper(
	cdc codec.Codec,
	storeKey,
	memKey storetypes.StoreKey,
	bankKeeper types.BankKeeper,
	transferKeeper types.TransferKeeper,
	gravityKeeper types.GravityKeeper,
	evmKeeper types.EvmKeeper,
	accountKeeper types.AccountKeeper,
	authority string,
	// this line is used by starport scaffolding # ibc/keeper/parameter
) *Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(err)
	}

	return &Keeper{
		cdc:            cdc,
		storeKey:       storeKey,
		memKey:         memKey,
		bankKeeper:     bankKeeper,
		transferKeeper: transferKeeper,
		gravityKeeper:  gravityKeeper,
		evmKeeper:      evmKeeper,
		accountKeeper:  accountKeeper,
		authority:      authority,
		// this line is used by starport scaffolding # ibc/keeper/return
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// getExternalContractByDenom find the corresponding external contract for the denom,
func (k Keeper) getExternalContractByDenom(ctx sdk.Context, denom string) (common.Address, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.DenomToExternalContractKey(denom))
	if len(bz) == 0 {
		return common.Address{}, false
	}

	return common.BytesToAddress(bz), true
}

// getAutoContractByDenom find the corresponding auto-deployed contract for the denom,
func (k Keeper) getAutoContractByDenom(ctx sdk.Context, denom string) (common.Address, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.DenomToAutoContractKey(denom))
	if len(bz) == 0 {
		return common.Address{}, false
	}

	return common.BytesToAddress(bz), true
}

// GetAuthority returns the x/cronos module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// GetContractByDenom find the corresponding contract for the denom,
// external contract is taken in preference to auto-deployed one
func (k Keeper) GetContractByDenom(ctx sdk.Context, denom string) (contract common.Address, found bool) {
	contract, found = k.getExternalContractByDenom(ctx, denom)
	if !found {
		contract, found = k.getAutoContractByDenom(ctx, denom)
	}
	return
}

// GetDenomByContract find native denom by contract address
func (k Keeper) GetDenomByContract(ctx sdk.Context, contract common.Address) (denom string, found bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ContractToDenomKey(contract.Bytes()))
	if len(bz) == 0 {
		return "", false
	}

	return string(bz), true
}

// SetExternalContractForDenom set the external contract for native denom, replace the old one if any existing.
func (k Keeper) SetExternalContractForDenom(ctx sdk.Context, denom string, address common.Address) error {
	// check the contract is not registered already
	_, found := k.GetDenomByContract(ctx, address)
	if found {
		return fmt.Errorf("the contract is already registered: %s", address.Hex())
	}

	store := ctx.KVStore(k.storeKey)
	existing, found := k.getExternalContractByDenom(ctx, denom)
	if found {
		// remove existing mapping
		store.Delete(types.ContractToDenomKey(existing.Bytes()))
	}
	store.Set(types.DenomToExternalContractKey(denom), address.Bytes())
	store.Set(types.ContractToDenomKey(address.Bytes()), []byte(denom))
	return nil
}

// GetExternalContracts returns all external contract mappings
func (k Keeper) GetExternalContracts(ctx sdk.Context) (out []types.TokenMapping) {
	store := ctx.KVStore(k.storeKey)
	iter := prefix.NewStore(store, types.KeyPrefixDenomToExternalContract).Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		out = append(out, types.TokenMapping{
			Denom:    string(iter.Key()),
			Contract: common.BytesToAddress(iter.Value()).Hex(),
		})
	}
	return
}

// GetAutoContracts returns all auto-deployed contract mappings
func (k Keeper) GetAutoContracts(ctx sdk.Context) (out []types.TokenMapping) {
	store := ctx.KVStore(k.storeKey)
	iter := prefix.NewStore(store, types.KeyPrefixDenomToAutoContract).Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		out = append(out, types.TokenMapping{
			Denom:    string(iter.Key()),
			Contract: common.BytesToAddress(iter.Value()).Hex(),
		})
	}
	return
}

// DeleteExternalContractForDenom delete the external contract mapping for native denom,
// returns false if mapping not exists.
func (k Keeper) DeleteExternalContractForDenom(ctx sdk.Context, denom string) bool {
	store := ctx.KVStore(k.storeKey)
	contract, found := k.getExternalContractByDenom(ctx, denom)
	if !found {
		return false
	}
	store.Delete(types.DenomToExternalContractKey(denom))
	store.Delete(types.ContractToDenomKey(contract.Bytes()))
	return true
}

// SetAutoContractForDenom set the auto deployed contract for native denom
func (k Keeper) SetAutoContractForDenom(ctx sdk.Context, denom string, address common.Address) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.DenomToAutoContractKey(denom), address.Bytes())
	store.Set(types.ContractToDenomKey(address.Bytes()), []byte(denom))
}

// OnRecvVouchers try to convert ibc voucher to evm coins, revert the state in case of failure
func (k Keeper) OnRecvVouchers(
	ctx sdk.Context,
	tokens sdk.Coins,
	receiver string,
) {
	cacheCtx, commit := ctx.CacheContext()
	err := k.ConvertVouchersToEvmCoins(cacheCtx, receiver, tokens)
	if err == nil {
		commit()
		ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
	} else {
		k.Logger(ctx).Error(
			fmt.Sprintf("Failed to convert vouchers to evm tokens for receiver %s, coins %s. Receive error %s",
				receiver, tokens.String(), err))
	}
}

func (k Keeper) GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI {
	return k.accountKeeper.GetAccount(ctx, addr)
}

// RegisterOrUpdateTokenMapping update the token mapping, register a coin metadata if needed
func (k Keeper) RegisterOrUpdateTokenMapping(ctx sdk.Context, msg *types.MsgUpdateTokenMapping) error {
	if types.IsSourceCoin(msg.Denom) {
		contract, err := types.GetContractAddressFromDenom(msg.Denom)
		if err != nil {
			return err
		}
		// we check that denom use the same contract address in checksum format
		if contract != common.HexToAddress(msg.Contract).Hex() {
			return sdkerrors.Wrapf(
				sdkerrors.ErrInvalidRequest,
				"coin denom %s does not match with contract address %s",
				msg.Denom, common.HexToAddress(msg.Contract).Hex())
		}

		// check that the coin is registered, otherwise register it
		metadata, exist := k.bankKeeper.GetDenomMetaData(ctx, msg.Denom)
		if !exist {
			// create new metadata
			metadata = banktypes.Metadata{
				Base: msg.Denom,
				Name: msg.Denom,
			}
		}
		// update existing metadata
		metadata.Symbol = msg.Symbol
		metadata.Display = strings.ToLower(msg.Symbol)
		if msg.Decimal != 0 {
			metadata.DenomUnits = []*banktypes.DenomUnit{
				{
					Denom:    metadata.Base,
					Exponent: 0,
				},
				{
					Denom:    metadata.Display,
					Exponent: msg.Decimal,
				},
			}
		} else {
			metadata.DenomUnits = []*banktypes.DenomUnit{
				{
					Denom:    metadata.Base,
					Exponent: 0,
				},
			}
		}
		k.bankKeeper.SetDenomMetaData(ctx, metadata)

		// update the mapping
		if err := k.SetExternalContractForDenom(ctx, msg.Denom, common.HexToAddress(contract)); err != nil {
			return err
		}
	} else {
		if len(msg.Contract) == 0 {
			// delete existing mapping
			k.DeleteExternalContractForDenom(ctx, msg.Denom)
		} else {
			if !common.IsHexAddress(msg.Contract) {
				return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid contract address (%s)", msg.Contract)
			}
			// update the mapping
			contract := common.HexToAddress(msg.Contract)
			if err := k.SetExternalContractForDenom(ctx, msg.Denom, contract); err != nil {
				return err
			}
		}
	}

	return nil
}
