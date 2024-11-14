package events

import (
	"fmt"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

const maxIndexedArgs = 3

// EventDescriptor describes how to convert an native event to a eth log
type EventDescriptor struct {
	id         common.Hash
	indexed    []string
	nonIndexed []string
	packValues func([]interface{}) ([]byte, error)
}

func NewEventDescriptors(a abi.ABI) map[string]*EventDescriptor {
	descriptors := make(map[string]*EventDescriptor, len(a.Events))
	for _, event := range a.Events {
		event_type := toUnderScore(event.Name)
		descriptors[event_type] = &EventDescriptor{
			id:         event.ID,
			indexed:    getArguments(event.Inputs, true),
			nonIndexed: getArguments(event.Inputs, false),
			packValues: event.Inputs.NonIndexed().PackValues,
		}
	}
	return descriptors
}

func makeFilter(
	valueDecoders ValueDecoders,
	attrs map[string]string,
	attrNames []string,
	indexed bool,
) ([]any, error) {
	results := make([]any, 0, len(attrNames))
	for _, name := range attrNames {
		value, ok := attrs[name]
		if !ok {
			return nil, fmt.Errorf("attribute %s not found", name)
		}
		decode, ok := valueDecoders.GetDecoder(name)
		if !ok {
			return nil, fmt.Errorf("no decoder for %s", name)
		}
		values, err := decode(value, indexed)
		if err != nil {
			return nil, fmt.Errorf("failed to decode %s: %w", name, err)
		}
		results = append(results, values...)
	}
	return results, nil
}

func (desc *EventDescriptor) ConvertEvent(
	event []abci.EventAttribute,
	valueDecoders ValueDecoders,
	replaceAttrs map[string]string,
) (*ethtypes.Log, error) {
	attrs := make(map[string]string, len(event))
	for _, attr := range event {
		attrs[toUnderScore(attr.Key)] = attr.Value
	}
	for k, v := range replaceAttrs {
		attrs[k] = attrs[v]
	}
	filterQuery, err := makeFilter(valueDecoders, attrs, desc.indexed, true)
	if err != nil {
		return nil, err
	}
	filterQuery = append(
		[]any{desc.id},
		filterQuery...,
	)

	topics, err := abi.MakeTopics(filterQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to make topics: %w", err)
	}

	attrVals, err := makeFilter(valueDecoders, attrs, desc.nonIndexed, false)
	if err != nil {
		return nil, err
	}

	data, err := desc.packValues(attrVals)
	if err != nil {
		return nil, fmt.Errorf("failed to pack values: %w", err)
	}
	return &ethtypes.Log{
		Topics: topics[0],
		Data:   data,
	}, nil
}

func toUnderScore(input string) string {
	var output string
	for i, s := range input {
		if i > 0 && s >= 'A' && s <= 'Z' {
			output += "_"
		}
		output += string(s)
	}
	return strings.ToLower(output)
}

func getArguments(args abi.Arguments, indexed bool) []string {
	var result []string
	for _, arg := range args {
		if arg.Indexed == indexed {
			result = append(result, toUnderScore(arg.Name))
		}
	}

	if indexed && len(result) > maxIndexedArgs {
		panic("too many indexed args")
	}

	return result
}
