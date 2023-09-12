package precompiles

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	coretypes "github.com/ethereum/go-ethereum/core/types"
)

const maxIndexedArgs = 3

var (
	ErrEthEventNotRegistered = errors.New("eth event not registered")
	ErrNoAttributeKeyFound   = errors.New("this Ethereum event argument has no matching Cosmos attribute key")
	ErrNoValueDecoderFunc    = errors.New("no value decoder function is found for event attribute key")
)

type Methods []*Method

type Executable func(
	ctx context.Context,
	caller common.Address,
	value *big.Int,
	readonly bool,
	args ...any,
) (ret []any, err error)

type Method struct {
	AbiMethod   *abi.Method
	AbiSig      string
	Execute     Executable
	RequiredGas uint64
}

type (
	ValueDecoder  func(attributeValue string, indexed bool) (ethPrimitives []any, err error)
	ValueDecoders map[string]ValueDecoder
)

type Registrable interface {
	RegistryKey() common.Address
	ABIEvents() map[string]abi.Event
	CustomValueDecoders() ValueDecoders
}

type precompileLog struct {
	eventType        string
	precompileAddr   common.Address
	id               common.Hash
	indexedInputs    abi.Arguments
	nonIndexedInputs abi.Arguments
}

func newPrecompileLog(precompileAddr common.Address, abiEvent abi.Event) *precompileLog {
	return &precompileLog{
		eventType:        ToUnderScore(abiEvent.Name),
		precompileAddr:   precompileAddr,
		id:               abiEvent.ID,
		indexedInputs:    GetIndexed(abiEvent.Inputs),
		nonIndexedInputs: abiEvent.Inputs.NonIndexed(),
	}
}

func (l *precompileLog) RegistryKey() string {
	return l.eventType
}

type Factory struct {
	events              map[string]*precompileLog
	customValueDecoders ValueDecoders
}

func NewFactory(precompiles []Registrable) *Factory {
	f := &Factory{
		events:              make(map[string]*precompileLog),
		customValueDecoders: make(ValueDecoders),
	}
	f.registerAllEvents(precompiles)
	return f
}

func (f *Factory) Build(event *sdk.Event, height uint64) (*coretypes.Log, error) {
	pl, ok := f.events[event.Type]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrEthEventNotRegistered, event.Type)
	}

	var err error
	if err = validateAttributes(pl, event); err != nil {
		return nil, err
	}
	log := &coretypes.Log{
		Address:     pl.precompileAddr,
		BlockNumber: height,
	}
	if log.Topics, err = f.makeTopics(pl, event); err != nil {
		return nil, err
	}
	if log.Data, err = f.makeData(pl, event); err != nil {
		return nil, err
	}

	return log, nil
}

func (f *Factory) registerAllEvents(precompiles []Registrable) {
	for _, spc := range precompiles {
		moduleEthAddr := spc.RegistryKey()
		for _, event := range spc.ABIEvents() {
			item := newPrecompileLog(moduleEthAddr, event)
			f.events[item.RegistryKey()] = item
		}
		for attr, decoder := range spc.CustomValueDecoders() {
			f.customValueDecoders[attr] = decoder
		}
	}
}

func (f *Factory) makeTopics(pl *precompileLog, event *sdk.Event) ([]common.Hash, error) {
	filterQuery := make([]any, 0, len(pl.indexedInputs)+1)
	filterQuery = append(filterQuery, pl.id)
	for _, arg := range pl.indexedInputs {
		attrIdx := searchAttributesForArg(&event.Attributes, arg.Name)
		if attrIdx == -1 {
			return nil, fmt.Errorf("%w: %s", ErrNoAttributeKeyFound, arg.Name)
		}
		attr := &event.Attributes[attrIdx]
		decode, err := f.getValueDecoder(attr.Key)
		if err != nil {
			return nil, err
		}
		values, err := decode(attr.Value, true)
		if err != nil {
			return nil, err
		}
		filterQuery = append(filterQuery, values...)
	}

	topics, err := abi.MakeTopics(filterQuery)
	if err != nil {
		return nil, err
	}
	return topics[0], nil
}

func (f *Factory) makeData(pl *precompileLog, event *sdk.Event) ([]byte, error) {
	attrVals := make([]any, 0, len(pl.nonIndexedInputs))
	for _, arg := range pl.nonIndexedInputs {
		attrIdx := searchAttributesForArg(&event.Attributes, arg.Name)
		if attrIdx == -1 {
			return nil, fmt.Errorf("%w: %s", ErrNoAttributeKeyFound, arg.Name)
		}
		attr := event.Attributes[attrIdx]
		decode, err := f.getValueDecoder(attr.Key)
		if err != nil {
			return nil, err
		}
		values, err := decode(attr.Value, false)
		if err != nil {
			return nil, err
		}
		attrVals = append(attrVals, values...)
	}
	data, err := pl.nonIndexedInputs.PackValues(attrVals)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func ReturnStringAsIs(attributeValue string, _ bool) ([]any, error) {
	return []any{attributeValue}, nil
}

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

var defaultCosmosValueDecoders = ValueDecoders{
	banktypes.AttributeKeyRecipient: ConvertAccAddressFromBech32,
	banktypes.AttributeKeySpender:   ConvertAccAddressFromBech32,
	banktypes.AttributeKeyReceiver:  ConvertAccAddressFromBech32,
	banktypes.AttributeKeySender:    ConvertAccAddressFromBech32,
	banktypes.AttributeKeyMinter:    ConvertAccAddressFromBech32,
	banktypes.AttributeKeyBurner:    ConvertAccAddressFromBech32,
}

func (f *Factory) getValueDecoder(attrKey string) (ValueDecoder, error) {
	if customDecoder, found := f.customValueDecoders[attrKey]; found {
		return customDecoder, nil
	}
	if defaultDecoder, found := defaultCosmosValueDecoders[attrKey]; found {
		return defaultDecoder, nil
	}
	return ReturnStringAsIs, nil
}
