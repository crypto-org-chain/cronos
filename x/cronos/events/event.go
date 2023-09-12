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

func (desc *EventDescriptor) ConvertEvent(
	event []abci.EventAttribute,
	valueDecoders ValueDecoders,
) (*ethtypes.Log, error) {
	attrs := make(map[string]string, len(event))
	for _, attr := range event {
		attrs[attr.Key] = attr.Value
	}

	filterQuery := make([]any, 0, len(desc.indexed)+1)
	filterQuery = append(filterQuery, desc.id)
	for _, name := range desc.indexed {
		value, ok := attrs[name]
		if !ok {
			return nil, fmt.Errorf("attribute %s not found", name)
		}
		decode, ok := valueDecoders[name]
		if !ok {
			return nil, fmt.Errorf("value decoder for %s not found", name)
		}
		values, err := decode(value, true)
		if err != nil {
			return nil, fmt.Errorf("failed to decode %s: %w", name, err)
		}
		filterQuery = append(filterQuery, values...)
	}

	topics, err := abi.MakeTopics(filterQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to make topics: %w", err)
	}

	attrVals := make([]any, 0, len(desc.nonIndexed))
	for _, name := range desc.nonIndexed {
		value, ok := attrs[name]
		if !ok {
			return nil, fmt.Errorf("attribute %s not found", name)
		}

		decode, ok := valueDecoders[name]
		if !ok {
			return nil, fmt.Errorf("value decoder for %s not found", name)
		}

		values, err := decode(value, false)
		if err != nil {
			return nil, fmt.Errorf("failed to decode %s: %w", name, err)
		}
		attrVals = append(attrVals, values...)
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
		if arg.Indexed && indexed {
			result = append(result, toUnderScore(arg.Name))
		} else {
		}
	}

	if indexed && len(result) > maxIndexedArgs {
		panic("too many indexed args")
	}

	return result
}
