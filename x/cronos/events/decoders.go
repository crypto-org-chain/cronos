package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/lib"
	generated "github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/relayer"
)

const intBase = 10

func AccAddressFromBech32(address string) (addr sdk.AccAddress, err error) {
	if len(strings.TrimSpace(address)) == 0 {
		return sdk.AccAddress{}, errors.New("empty address string is not allowed")
	}
	_, bz, err := bech32.DecodeAndConvert(address)
	if err != nil {
		return nil, err
	}
	// skip invalid Bech32 prefix check for cross chain
	err = sdk.VerifyAddressFormat(bz)
	if err != nil {
		return nil, err
	}
	return sdk.AccAddress(bz), nil
}

func ConvertAccAddressFromBech32(attributeValue string, _ bool) ([]any, error) {
	accAddress, err := AccAddressFromBech32(attributeValue)
	if err == nil {
		return []any{common.BytesToAddress(accAddress)}, nil
	}
	return []any{attributeValue}, nil
}

func ConvertAmount(attributeValue string, indexed bool) ([]any, error) {
	coins, err := sdk.ParseCoinsNormalized(attributeValue)
	if err == nil {
		return []any{sdkCoinsToEvmCoins(coins)}, nil
	}
	amt, ok := new(big.Int).SetString(attributeValue, intBase)
	if !ok {
		return nil, fmt.Errorf("failed to parse amount: %v", attributeValue)
	}
	return []any{amt}, nil
}

func sdkCoinsToEvmCoins(sdkCoins sdk.Coins) []lib.CosmosCoin {
	evmCoins := make([]lib.CosmosCoin, len(sdkCoins))
	for i, coin := range sdkCoins {
		evmCoins[i] = lib.CosmosCoin{
			Amount: coin.Amount.BigInt(),
			Denom:  coin.Denom,
		}
	}
	return evmCoins
}
func ConvertPacketData(attributeValue string, indexed bool) ([]any, error) {
	var tokenPacketData transfertypes.FungibleTokenPacketData
	err := json.Unmarshal([]byte(attributeValue), &tokenPacketData)
	if err != nil {
		return nil, err
	}
	receiver, err := convertAddress(tokenPacketData.Receiver)
	if err != nil {
		return nil, err
	}
	if indexed {
		return []any{
			tokenPacketData.Sender,
			receiver.String(),
		}, nil
	}
	amt, ok := new(big.Int).SetString(tokenPacketData.Amount, intBase)
	if !ok {
		return nil, errors.New("invalid amount")
	}
	return []any{
		generated.IRelayerModulePacketData{
			Receiver: *receiver,
			Sender:   tokenPacketData.Sender,
			Amount: []generated.CosmosCoin{
				{
					Amount: amt,
					Denom:  tokenPacketData.Denom,
				},
			},
		},
	}, nil
}

func convertAddress(addrString string) (*common.Address, error) {
	cfg := sdk.GetConfig()
	var addr []byte
	// try hex, then bech32
	switch {
	case common.IsHexAddress(addrString):
		addr = common.HexToAddress(addrString).Bytes()
	case strings.HasPrefix(addrString, cfg.GetBech32ValidatorAddrPrefix()):
		addr, _ = sdk.ValAddressFromBech32(addrString)
	case strings.HasPrefix(addrString, cfg.GetBech32AccountAddrPrefix()):
		addr, _ = sdk.AccAddressFromBech32(addrString)
	default:
		return nil, fmt.Errorf("expected a valid hex or bech32 address (acc prefix %s), got '%s'", cfg.GetBech32AccountAddrPrefix(), addrString)
	}
	to := common.BytesToAddress(addr)
	return &to, nil
}
