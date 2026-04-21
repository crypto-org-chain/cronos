package keeper

import (
	"fmt"
	"math/big"
	"strings"

	ibccallbacktypes "github.com/cosmos/ibc-go/v10/modules/apps/callbacks/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	cronosprecompiles "github.com/crypto-org-chain/cronos/x/cronos/keeper/precompiles"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	ethermint "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"cosmossdk.io/errors"
	"cosmossdk.io/log"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
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

var _ ibccallbacktypes.ContractKeeper = Keeper{}

func NewKeeper(
	cdc codec.Codec,
	storeKey,
	memKey storetypes.StoreKey,
	bankKeeper types.BankKeeper,
	transferKeeper types.TransferKeeper,
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
	return contract, found
}

// GetDenomByContract find native denom by contract address
func (k Keeper) GetDenomByContract(ctx sdk.Context, contract common.Address) (denom string, found bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ContractToDenomKey(contract.Bytes()))
	if len(bz) == 0 {
		return "", false
	}
	denom = string(bz)
	// Cross-check against current mapping to avoid stale reverse entries in legacy state.
	current, ok := k.GetContractByDenom(ctx, denom)
	if !ok || current != contract {
		return "", false
	}
	return denom, true
}

func (k Keeper) contractOwnedByDenom(ctx sdk.Context, denom string, address common.Address) bool {
	if ext, found := k.getExternalContractByDenom(ctx, denom); found && ext == address {
		return true
	}
	if auto, found := k.getAutoContractByDenom(ctx, denom); found && auto == address {
		return true
	}
	return false
}

func (k Keeper) ensureContractNotMapped(ctx sdk.Context, denom string, address common.Address) error {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ContractToDenomKey(address.Bytes()))
	if len(bz) == 0 {
		return nil
	}
	existingDenom := string(bz)
	if existingDenom == denom {
		return nil
	}
	if k.contractOwnedByDenom(ctx, existingDenom, address) {
		return errors.Wrapf(types.ErrContractAlreadyRegistered, "contract %s is already registered for denom %s", address.Hex(), existingDenom)
	}
	// stale reverse entry
	store.Delete(types.ContractToDenomKey(address.Bytes()))
	return nil
}

func deleteReverseIfOwned(store storetypes.KVStore, address common.Address, denom string) {
	if bz := store.Get(types.ContractToDenomKey(address.Bytes())); len(bz) != 0 && string(bz) == denom {
		store.Delete(types.ContractToDenomKey(address.Bytes()))
	}
}

// validateCRC21Target rejects contract addresses that cannot be valid CRC21
// targets, preventing IBC voucher conversion from silently stranding funds at
// zero / precompile / EOA bech32 addresses.
func (k Keeper) validateCRC21Target(ctx sdk.Context, addr common.Address) error {
	if addr == (common.Address{}) {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "crc21 contract must not be zero address")
	}
	// geth precompiles live in 0x01..0xff; any address strictly below 0x0100
	// is either the zero address or a precompile and cannot hold bytecode.
	if addr.Big().Cmp(big.NewInt(256)) < 0 {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress,
			"crc21 contract must not be in precompile range: %s", addr.Hex())
	}
	acct := k.accountKeeper.GetAccount(ctx, sdk.AccAddress(addr.Bytes()))
	ethAcct, ok := acct.(ethermint.EthAccountI)
	if !ok || evmtypes.IsEmptyCodeHash(ethAcct.GetCodeHash().Bytes()) {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress,
			"crc21 contract has no bytecode: %s", addr.Hex())
	}
	return nil
}

// SetExternalContractForDenom set the external contract for native denom, replace the old one if any existing.
func (k Keeper) SetExternalContractForDenom(ctx sdk.Context, denom string, address common.Address) error {
	// check the contract is not registered already
	if err := k.ensureContractNotMapped(ctx, denom, address); err != nil {
		return err
	}

	store := ctx.KVStore(k.storeKey)
	existing, found := k.getExternalContractByDenom(ctx, denom)
	if found {
		// remove existing mapping
		deleteReverseIfOwned(store, existing, denom)
	}
	if !types.IsSourceCoin(denom) {
		auto, found := k.getAutoContractByDenom(ctx, denom)
		if found {
			// retire auto mapping when external mapping is set for non-source denoms
			store.Delete(types.DenomToAutoContractKey(denom))
			deleteReverseIfOwned(store, auto, denom)
		}
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
	return out
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
	return out
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
	deleteReverseIfOwned(store, contract, denom)
	if auto, found := k.getAutoContractByDenom(ctx, denom); found {
		bz := store.Get(types.ContractToDenomKey(auto.Bytes()))
		if len(bz) == 0 {
			store.Set(types.ContractToDenomKey(auto.Bytes()), []byte(denom))
		} else if existingDenom := string(bz); existingDenom != denom {
			if k.contractOwnedByDenom(ctx, existingDenom, auto) {
				// auto address is already owned by another denom; drop local auto mapping
				store.Delete(types.DenomToAutoContractKey(denom))
			} else {
				// stale reverse entry
				store.Set(types.ContractToDenomKey(auto.Bytes()), []byte(denom))
			}
		}
	}
	return true
}

// SetAutoContractForDenom set the auto deployed contract for native denom
func (k Keeper) SetAutoContractForDenom(ctx sdk.Context, denom string, address common.Address) error {
	store := ctx.KVStore(k.storeKey)
	if _, found := k.getExternalContractByDenom(ctx, denom); found && !types.IsSourceCoin(denom) {
		return errors.Wrapf(types.ErrExternalMappingExists, "external mapping already exists for denom %s", denom)
	}
	if err := k.ensureContractNotMapped(ctx, denom, address); err != nil {
		return err
	}
	store.Set(types.DenomToAutoContractKey(denom), address.Bytes())
	store.Set(types.ContractToDenomKey(address.Bytes()), []byte(denom))
	return nil
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
	} else {
		k.Logger(ctx).Error(
			fmt.Sprintf("Failed to convert vouchers to evm tokens for receiver %s, coins %s. Receive error %s",
				receiver, tokens.String(), err))
	}
}

func (k Keeper) GetAccount(ctx sdk.Context, addr sdk.AccAddress) sdk.AccountI {
	return k.accountKeeper.GetAccount(ctx, addr)
}

func (k Keeper) ensureContractCode(ctx sdk.Context, contract common.Address) error {
	resp, err := k.evmKeeper.Code(ctx, &evmtypes.QueryCodeRequest{
		Address: contract.Hex(),
	})
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "failed to query contract code (%s): %v", contract.Hex(), err)
	}
	if resp == nil || len(resp.Code) == 0 {
		return errors.Wrapf(sdkerrors.ErrInvalidRequest, "no contract code at address (%s)", contract.Hex())
	}
	return nil
}

// RegisterOrUpdateTokenMapping update the token mapping, register a coin metadata if needed
func (k Keeper) RegisterOrUpdateTokenMapping(ctx sdk.Context, msg *types.MsgUpdateTokenMapping) error {
	if types.IsSourceCoin(msg.Denom) {
		_, err := types.GetContractAddressFromDenom(msg.Denom)
		if err != nil {
			return err
		}

		if !common.IsHexAddress(msg.Contract) {
			return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid contract address (%s)", msg.Contract)
		}
		contract := common.HexToAddress(msg.Contract)
		if err := k.validateCRC21Target(ctx, contract); err != nil {
			return err
		}
		if err := k.ensureContractCode(ctx, contract); err != nil {
			return err
		}
		if err := k.SetExternalContractForDenom(ctx, msg.Denom, contract); err != nil {
			return err
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
	} else {
		if len(msg.Contract) == 0 {
			// delete existing mapping
			k.DeleteExternalContractForDenom(ctx, msg.Denom)
		} else {
			if !common.IsHexAddress(msg.Contract) {
				return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid contract address (%s)", msg.Contract)
			}
			// update the mapping
			contract := common.HexToAddress(msg.Contract)
			if err := k.ensureContractCode(ctx, contract); err != nil {
				return err
			}
			if err := k.SetExternalContractForDenom(ctx, msg.Denom, contract); err != nil {
				return err
			}
		}
	}

	return nil
}

func (k Keeper) onPacketResult(
	ctx sdk.Context,
	packet channeltypes.Packet,
	acknowledgement bool,
	relayer sdk.AccAddress,
	contractAddress,
	packetSenderAddress string,
) error {
	sender, err := sdk.AccAddressFromBech32(packetSenderAddress)
	if err != nil {
		return fmt.Errorf("invalid bech32 address: %s, err: %w", packetSenderAddress, err)
	}
	senderAddr := common.BytesToAddress(sender)
	contractAddr := common.HexToAddress(contractAddress)
	if senderAddr != contractAddr {
		return fmt.Errorf("sender is not authenticated: expected %s, got %s", senderAddr, contractAddr)
	}
	data, err := cronosprecompiles.OnPacketResultCallback(packet.SourceChannel, packet.Sequence, acknowledgement)
	if err != nil {
		return err
	}
	gasLimit := k.GetParams(ctx).MaxCallbackGas
	_, res, err := k.CallEVM(ctx, &senderAddr, data, big.NewInt(0), gasLimit)
	if err != nil {
		return err
	}
	if res.Failed() {
		return fmt.Errorf("IBC callback EVM execution reverted: %s", res.VmError)
	}
	return nil
}

func (k Keeper) IBCOnAcknowledgementPacketCallback(
	ctx sdk.Context,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
	contractAddress,
	packetSenderAddress string,
	version string,
) error {
	var res channeltypes.Acknowledgement
	if err := k.cdc.UnmarshalJSON(acknowledgement, &res); err != nil {
		return err
	}
	return k.onPacketResult(ctx, packet, res.Success(), relayer, contractAddress, packetSenderAddress)
}

func (k Keeper) IBCOnTimeoutPacketCallback(
	ctx sdk.Context,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
	contractAddress,
	packetSenderAddress string,
	version string,
) error {
	return k.onPacketResult(ctx, packet, false, relayer, contractAddress, packetSenderAddress)
}

func (k Keeper) IBCReceivePacketCallback(
	ctx sdk.Context,
	packet ibcexported.PacketI,
	ack ibcexported.Acknowledgement,
	contractAddress string,
	version string,
) error {
	return nil
}

func (k Keeper) IBCSendPacketCallback(
	ctx sdk.Context,
	sourcePort string,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
	packetData []byte,
	contractAddress,
	packetSenderAddress string,
	version string,
) error {
	return nil
}

func (k Keeper) GetBlockList(ctx sdk.Context) []byte {
	return ctx.KVStore(k.storeKey).Get(types.KeyPrefixBlockList)
}
