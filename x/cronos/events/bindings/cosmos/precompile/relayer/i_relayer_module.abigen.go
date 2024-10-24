// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package relayer

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// CosmosCoin is an auto generated low-level Go binding around an user-defined struct.
type CosmosCoin struct {
	Amount *big.Int
	Denom  string
}

// IRelayerModulePacketData is an auto generated low-level Go binding around an user-defined struct.
type IRelayerModulePacketData struct {
	Receiver common.Address
	Sender   string
	Amount   []CosmosCoin
}

// RelayerModuleMetaData contains all meta data concerning the RelayerModule contract.
var RelayerModuleMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"packetSequence\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcPort\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetSrcPortInfo\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetSrcChannelInfo\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstPort\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"}],\"name\":\"AcknowledgePacket\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"burner\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"Burn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"ChannelClosed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"CoinReceived\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"CoinSpent\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"minter\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"Coinbase\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"name\":\"DenominationTrace\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"fee\",\"type\":\"string\"}],\"name\":\"DistributeFee\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"FungibleTokenPacket\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"IbcTransfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"packetSequence\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcPort\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetSrcPortInfo\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetSrcChannelInfo\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstPort\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"sender\",\"type\":\"string\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"indexed\":false,\"internalType\":\"structIRelayerModule.PacketData\",\"name\":\"packetDataHex\",\"type\":\"tuple\"}],\"name\":\"RecvPacket\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"refundReceiver\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"refundDenom\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"refundAmount\",\"type\":\"string\"}],\"name\":\"Timeout\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"packetSequence\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcPort\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetSrcPortInfo\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetSrcChannelInfo\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstPort\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"}],\"name\":\"TimeoutPacket\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"packetSequence\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcPort\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetSrcPortInfo\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetSrcChannelInfo\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstPort\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"sender\",\"type\":\"string\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"indexed\":false,\"internalType\":\"structIRelayerModule.PacketData\",\"name\":\"packetDataHex\",\"type\":\"tuple\"}],\"name\":\"WriteAcknowledgement\",\"type\":\"event\"}]",
}

// RelayerModuleABI is the input ABI used to generate the binding from.
// Deprecated: Use RelayerModuleMetaData.ABI instead.
var RelayerModuleABI = RelayerModuleMetaData.ABI

// RelayerModule is an auto generated Go binding around an Ethereum contract.
type RelayerModule struct {
	RelayerModuleCaller     // Read-only binding to the contract
	RelayerModuleTransactor // Write-only binding to the contract
	RelayerModuleFilterer   // Log filterer for contract events
}

// RelayerModuleCaller is an auto generated read-only Go binding around an Ethereum contract.
type RelayerModuleCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayerModuleTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RelayerModuleTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayerModuleFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RelayerModuleFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayerModuleSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RelayerModuleSession struct {
	Contract     *RelayerModule    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RelayerModuleCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RelayerModuleCallerSession struct {
	Contract *RelayerModuleCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// RelayerModuleTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RelayerModuleTransactorSession struct {
	Contract     *RelayerModuleTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// RelayerModuleRaw is an auto generated low-level Go binding around an Ethereum contract.
type RelayerModuleRaw struct {
	Contract *RelayerModule // Generic contract binding to access the raw methods on
}

// RelayerModuleCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RelayerModuleCallerRaw struct {
	Contract *RelayerModuleCaller // Generic read-only contract binding to access the raw methods on
}

// RelayerModuleTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RelayerModuleTransactorRaw struct {
	Contract *RelayerModuleTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRelayerModule creates a new instance of RelayerModule, bound to a specific deployed contract.
func NewRelayerModule(address common.Address, backend bind.ContractBackend) (*RelayerModule, error) {
	contract, err := bindRelayerModule(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RelayerModule{RelayerModuleCaller: RelayerModuleCaller{contract: contract}, RelayerModuleTransactor: RelayerModuleTransactor{contract: contract}, RelayerModuleFilterer: RelayerModuleFilterer{contract: contract}}, nil
}

// NewRelayerModuleCaller creates a new read-only instance of RelayerModule, bound to a specific deployed contract.
func NewRelayerModuleCaller(address common.Address, caller bind.ContractCaller) (*RelayerModuleCaller, error) {
	contract, err := bindRelayerModule(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleCaller{contract: contract}, nil
}

// NewRelayerModuleTransactor creates a new write-only instance of RelayerModule, bound to a specific deployed contract.
func NewRelayerModuleTransactor(address common.Address, transactor bind.ContractTransactor) (*RelayerModuleTransactor, error) {
	contract, err := bindRelayerModule(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleTransactor{contract: contract}, nil
}

// NewRelayerModuleFilterer creates a new log filterer instance of RelayerModule, bound to a specific deployed contract.
func NewRelayerModuleFilterer(address common.Address, filterer bind.ContractFilterer) (*RelayerModuleFilterer, error) {
	contract, err := bindRelayerModule(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleFilterer{contract: contract}, nil
}

// bindRelayerModule binds a generic wrapper to an already deployed contract.
func bindRelayerModule(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := RelayerModuleMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RelayerModule *RelayerModuleRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RelayerModule.Contract.RelayerModuleCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RelayerModule *RelayerModuleRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RelayerModule.Contract.RelayerModuleTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RelayerModule *RelayerModuleRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RelayerModule.Contract.RelayerModuleTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RelayerModule *RelayerModuleCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RelayerModule.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RelayerModule *RelayerModuleTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RelayerModule.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RelayerModule *RelayerModuleTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RelayerModule.Contract.contract.Transact(opts, method, params...)
}

// RelayerModuleAcknowledgePacketIterator is returned from FilterAcknowledgePacket and is used to iterate over the raw logs and unpacked data for AcknowledgePacket events raised by the RelayerModule contract.
type RelayerModuleAcknowledgePacketIterator struct {
	Event *RelayerModuleAcknowledgePacket // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleAcknowledgePacketIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleAcknowledgePacket)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleAcknowledgePacket)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleAcknowledgePacketIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleAcknowledgePacketIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleAcknowledgePacket represents a AcknowledgePacket event raised by the RelayerModule contract.
type RelayerModuleAcknowledgePacket struct {
	PacketSequence       *big.Int
	PacketSrcPort        common.Hash
	PacketSrcChannel     common.Hash
	PacketSrcPortInfo    string
	PacketSrcChannelInfo string
	PacketDstPort        string
	PacketDstChannel     string
	ConnectionId         string
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterAcknowledgePacket is a free log retrieval operation binding the contract event 0xa08a233528433f6c16f21b1264864dbb65aa78a26e16504065cd2a8d66fe40c3.
//
// Solidity: event AcknowledgePacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterAcknowledgePacket(opts *bind.FilterOpts, packetSequence []*big.Int, packetSrcPort []string, packetSrcChannel []string) (*RelayerModuleAcknowledgePacketIterator, error) {

	var packetSequenceRule []interface{}
	for _, packetSequenceItem := range packetSequence {
		packetSequenceRule = append(packetSequenceRule, packetSequenceItem)
	}
	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "AcknowledgePacket", packetSequenceRule, packetSrcPortRule, packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleAcknowledgePacketIterator{contract: _RelayerModule.contract, event: "AcknowledgePacket", logs: logs, sub: sub}, nil
}

// WatchAcknowledgePacket is a free log subscription operation binding the contract event 0xa08a233528433f6c16f21b1264864dbb65aa78a26e16504065cd2a8d66fe40c3.
//
// Solidity: event AcknowledgePacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchAcknowledgePacket(opts *bind.WatchOpts, sink chan<- *RelayerModuleAcknowledgePacket, packetSequence []*big.Int, packetSrcPort []string, packetSrcChannel []string) (event.Subscription, error) {

	var packetSequenceRule []interface{}
	for _, packetSequenceItem := range packetSequence {
		packetSequenceRule = append(packetSequenceRule, packetSequenceItem)
	}
	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "AcknowledgePacket", packetSequenceRule, packetSrcPortRule, packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleAcknowledgePacket)
				if err := _RelayerModule.contract.UnpackLog(event, "AcknowledgePacket", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAcknowledgePacket is a log parse operation binding the contract event 0xa08a233528433f6c16f21b1264864dbb65aa78a26e16504065cd2a8d66fe40c3.
//
// Solidity: event AcknowledgePacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseAcknowledgePacket(log types.Log) (*RelayerModuleAcknowledgePacket, error) {
	event := new(RelayerModuleAcknowledgePacket)
	if err := _RelayerModule.contract.UnpackLog(event, "AcknowledgePacket", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleBurnIterator is returned from FilterBurn and is used to iterate over the raw logs and unpacked data for Burn events raised by the RelayerModule contract.
type RelayerModuleBurnIterator struct {
	Event *RelayerModuleBurn // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleBurnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleBurn)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleBurn)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleBurnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleBurnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleBurn represents a Burn event raised by the RelayerModule contract.
type RelayerModuleBurn struct {
	Burner common.Address
	Amount []CosmosCoin
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterBurn is a free log retrieval operation binding the contract event 0x9fa0c2fb43a81906efbb089cd76002325d71b437612a2a987c707446629d6ab0.
//
// Solidity: event Burn(address indexed burner, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) FilterBurn(opts *bind.FilterOpts, burner []common.Address) (*RelayerModuleBurnIterator, error) {

	var burnerRule []interface{}
	for _, burnerItem := range burner {
		burnerRule = append(burnerRule, burnerItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "Burn", burnerRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleBurnIterator{contract: _RelayerModule.contract, event: "Burn", logs: logs, sub: sub}, nil
}

// WatchBurn is a free log subscription operation binding the contract event 0x9fa0c2fb43a81906efbb089cd76002325d71b437612a2a987c707446629d6ab0.
//
// Solidity: event Burn(address indexed burner, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) WatchBurn(opts *bind.WatchOpts, sink chan<- *RelayerModuleBurn, burner []common.Address) (event.Subscription, error) {

	var burnerRule []interface{}
	for _, burnerItem := range burner {
		burnerRule = append(burnerRule, burnerItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "Burn", burnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleBurn)
				if err := _RelayerModule.contract.UnpackLog(event, "Burn", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseBurn is a log parse operation binding the contract event 0x9fa0c2fb43a81906efbb089cd76002325d71b437612a2a987c707446629d6ab0.
//
// Solidity: event Burn(address indexed burner, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) ParseBurn(log types.Log) (*RelayerModuleBurn, error) {
	event := new(RelayerModuleBurn)
	if err := _RelayerModule.contract.UnpackLog(event, "Burn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleChannelClosedIterator is returned from FilterChannelClosed and is used to iterate over the raw logs and unpacked data for ChannelClosed events raised by the RelayerModule contract.
type RelayerModuleChannelClosedIterator struct {
	Event *RelayerModuleChannelClosed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleChannelClosedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleChannelClosed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleChannelClosed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleChannelClosedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleChannelClosedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleChannelClosed represents a ChannelClosed event raised by the RelayerModule contract.
type RelayerModuleChannelClosed struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterChannelClosed is a free log retrieval operation binding the contract event 0x6821b7df6ac3c960825ec594d716b8c7babb16a672ffdf0679a9ff6e873d5c82.
//
// Solidity: event ChannelClosed()
func (_RelayerModule *RelayerModuleFilterer) FilterChannelClosed(opts *bind.FilterOpts) (*RelayerModuleChannelClosedIterator, error) {

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ChannelClosed")
	if err != nil {
		return nil, err
	}
	return &RelayerModuleChannelClosedIterator{contract: _RelayerModule.contract, event: "ChannelClosed", logs: logs, sub: sub}, nil
}

// WatchChannelClosed is a free log subscription operation binding the contract event 0x6821b7df6ac3c960825ec594d716b8c7babb16a672ffdf0679a9ff6e873d5c82.
//
// Solidity: event ChannelClosed()
func (_RelayerModule *RelayerModuleFilterer) WatchChannelClosed(opts *bind.WatchOpts, sink chan<- *RelayerModuleChannelClosed) (event.Subscription, error) {

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ChannelClosed")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleChannelClosed)
				if err := _RelayerModule.contract.UnpackLog(event, "ChannelClosed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseChannelClosed is a log parse operation binding the contract event 0x6821b7df6ac3c960825ec594d716b8c7babb16a672ffdf0679a9ff6e873d5c82.
//
// Solidity: event ChannelClosed()
func (_RelayerModule *RelayerModuleFilterer) ParseChannelClosed(log types.Log) (*RelayerModuleChannelClosed, error) {
	event := new(RelayerModuleChannelClosed)
	if err := _RelayerModule.contract.UnpackLog(event, "ChannelClosed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleCoinReceivedIterator is returned from FilterCoinReceived and is used to iterate over the raw logs and unpacked data for CoinReceived events raised by the RelayerModule contract.
type RelayerModuleCoinReceivedIterator struct {
	Event *RelayerModuleCoinReceived // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleCoinReceivedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleCoinReceived)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleCoinReceived)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleCoinReceivedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleCoinReceivedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleCoinReceived represents a CoinReceived event raised by the RelayerModule contract.
type RelayerModuleCoinReceived struct {
	Receiver common.Address
	Amount   []CosmosCoin
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterCoinReceived is a free log retrieval operation binding the contract event 0x13f9c352919df1623a08e6d6d9eac5f774573896f09916d8fbc5d083095fc3b4.
//
// Solidity: event CoinReceived(address indexed receiver, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) FilterCoinReceived(opts *bind.FilterOpts, receiver []common.Address) (*RelayerModuleCoinReceivedIterator, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "CoinReceived", receiverRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleCoinReceivedIterator{contract: _RelayerModule.contract, event: "CoinReceived", logs: logs, sub: sub}, nil
}

// WatchCoinReceived is a free log subscription operation binding the contract event 0x13f9c352919df1623a08e6d6d9eac5f774573896f09916d8fbc5d083095fc3b4.
//
// Solidity: event CoinReceived(address indexed receiver, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) WatchCoinReceived(opts *bind.WatchOpts, sink chan<- *RelayerModuleCoinReceived, receiver []common.Address) (event.Subscription, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "CoinReceived", receiverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleCoinReceived)
				if err := _RelayerModule.contract.UnpackLog(event, "CoinReceived", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCoinReceived is a log parse operation binding the contract event 0x13f9c352919df1623a08e6d6d9eac5f774573896f09916d8fbc5d083095fc3b4.
//
// Solidity: event CoinReceived(address indexed receiver, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) ParseCoinReceived(log types.Log) (*RelayerModuleCoinReceived, error) {
	event := new(RelayerModuleCoinReceived)
	if err := _RelayerModule.contract.UnpackLog(event, "CoinReceived", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleCoinSpentIterator is returned from FilterCoinSpent and is used to iterate over the raw logs and unpacked data for CoinSpent events raised by the RelayerModule contract.
type RelayerModuleCoinSpentIterator struct {
	Event *RelayerModuleCoinSpent // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleCoinSpentIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleCoinSpent)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleCoinSpent)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleCoinSpentIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleCoinSpentIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleCoinSpent represents a CoinSpent event raised by the RelayerModule contract.
type RelayerModuleCoinSpent struct {
	Spender common.Address
	Amount  []CosmosCoin
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterCoinSpent is a free log retrieval operation binding the contract event 0x8b8b22fea5b121b174e6cfea34ddaf187b66b43dab67679fa291a0fae2427a99.
//
// Solidity: event CoinSpent(address indexed spender, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) FilterCoinSpent(opts *bind.FilterOpts, spender []common.Address) (*RelayerModuleCoinSpentIterator, error) {

	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "CoinSpent", spenderRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleCoinSpentIterator{contract: _RelayerModule.contract, event: "CoinSpent", logs: logs, sub: sub}, nil
}

// WatchCoinSpent is a free log subscription operation binding the contract event 0x8b8b22fea5b121b174e6cfea34ddaf187b66b43dab67679fa291a0fae2427a99.
//
// Solidity: event CoinSpent(address indexed spender, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) WatchCoinSpent(opts *bind.WatchOpts, sink chan<- *RelayerModuleCoinSpent, spender []common.Address) (event.Subscription, error) {

	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "CoinSpent", spenderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleCoinSpent)
				if err := _RelayerModule.contract.UnpackLog(event, "CoinSpent", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCoinSpent is a log parse operation binding the contract event 0x8b8b22fea5b121b174e6cfea34ddaf187b66b43dab67679fa291a0fae2427a99.
//
// Solidity: event CoinSpent(address indexed spender, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) ParseCoinSpent(log types.Log) (*RelayerModuleCoinSpent, error) {
	event := new(RelayerModuleCoinSpent)
	if err := _RelayerModule.contract.UnpackLog(event, "CoinSpent", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleCoinbaseIterator is returned from FilterCoinbase and is used to iterate over the raw logs and unpacked data for Coinbase events raised by the RelayerModule contract.
type RelayerModuleCoinbaseIterator struct {
	Event *RelayerModuleCoinbase // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleCoinbaseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleCoinbase)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleCoinbase)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleCoinbaseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleCoinbaseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleCoinbase represents a Coinbase event raised by the RelayerModule contract.
type RelayerModuleCoinbase struct {
	Minter common.Address
	Amount []CosmosCoin
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterCoinbase is a free log retrieval operation binding the contract event 0xefb3f1f2a9af64b5fcc2da3c5a088d780585c674b8075fe2a1ba6b0d906cbe9f.
//
// Solidity: event Coinbase(address indexed minter, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) FilterCoinbase(opts *bind.FilterOpts, minter []common.Address) (*RelayerModuleCoinbaseIterator, error) {

	var minterRule []interface{}
	for _, minterItem := range minter {
		minterRule = append(minterRule, minterItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "Coinbase", minterRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleCoinbaseIterator{contract: _RelayerModule.contract, event: "Coinbase", logs: logs, sub: sub}, nil
}

// WatchCoinbase is a free log subscription operation binding the contract event 0xefb3f1f2a9af64b5fcc2da3c5a088d780585c674b8075fe2a1ba6b0d906cbe9f.
//
// Solidity: event Coinbase(address indexed minter, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) WatchCoinbase(opts *bind.WatchOpts, sink chan<- *RelayerModuleCoinbase, minter []common.Address) (event.Subscription, error) {

	var minterRule []interface{}
	for _, minterItem := range minter {
		minterRule = append(minterRule, minterItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "Coinbase", minterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleCoinbase)
				if err := _RelayerModule.contract.UnpackLog(event, "Coinbase", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCoinbase is a log parse operation binding the contract event 0xefb3f1f2a9af64b5fcc2da3c5a088d780585c674b8075fe2a1ba6b0d906cbe9f.
//
// Solidity: event Coinbase(address indexed minter, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) ParseCoinbase(log types.Log) (*RelayerModuleCoinbase, error) {
	event := new(RelayerModuleCoinbase)
	if err := _RelayerModule.contract.UnpackLog(event, "Coinbase", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleDenominationTraceIterator is returned from FilterDenominationTrace and is used to iterate over the raw logs and unpacked data for DenominationTrace events raised by the RelayerModule contract.
type RelayerModuleDenominationTraceIterator struct {
	Event *RelayerModuleDenominationTrace // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleDenominationTraceIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleDenominationTrace)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleDenominationTrace)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleDenominationTraceIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleDenominationTraceIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleDenominationTrace represents a DenominationTrace event raised by the RelayerModule contract.
type RelayerModuleDenominationTrace struct {
	Denom string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterDenominationTrace is a free log retrieval operation binding the contract event 0x483180a024351f3ea4c4782eaadb34add715974648a3d47bbff4a7b76da20859.
//
// Solidity: event DenominationTrace(string denom)
func (_RelayerModule *RelayerModuleFilterer) FilterDenominationTrace(opts *bind.FilterOpts) (*RelayerModuleDenominationTraceIterator, error) {

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "DenominationTrace")
	if err != nil {
		return nil, err
	}
	return &RelayerModuleDenominationTraceIterator{contract: _RelayerModule.contract, event: "DenominationTrace", logs: logs, sub: sub}, nil
}

// WatchDenominationTrace is a free log subscription operation binding the contract event 0x483180a024351f3ea4c4782eaadb34add715974648a3d47bbff4a7b76da20859.
//
// Solidity: event DenominationTrace(string denom)
func (_RelayerModule *RelayerModuleFilterer) WatchDenominationTrace(opts *bind.WatchOpts, sink chan<- *RelayerModuleDenominationTrace) (event.Subscription, error) {

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "DenominationTrace")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleDenominationTrace)
				if err := _RelayerModule.contract.UnpackLog(event, "DenominationTrace", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDenominationTrace is a log parse operation binding the contract event 0x483180a024351f3ea4c4782eaadb34add715974648a3d47bbff4a7b76da20859.
//
// Solidity: event DenominationTrace(string denom)
func (_RelayerModule *RelayerModuleFilterer) ParseDenominationTrace(log types.Log) (*RelayerModuleDenominationTrace, error) {
	event := new(RelayerModuleDenominationTrace)
	if err := _RelayerModule.contract.UnpackLog(event, "DenominationTrace", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleDistributeFeeIterator is returned from FilterDistributeFee and is used to iterate over the raw logs and unpacked data for DistributeFee events raised by the RelayerModule contract.
type RelayerModuleDistributeFeeIterator struct {
	Event *RelayerModuleDistributeFee // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleDistributeFeeIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleDistributeFee)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleDistributeFee)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleDistributeFeeIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleDistributeFeeIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleDistributeFee represents a DistributeFee event raised by the RelayerModule contract.
type RelayerModuleDistributeFee struct {
	Receiver common.Address
	Fee      string
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterDistributeFee is a free log retrieval operation binding the contract event 0x67e2bceb7881996b4bbddf9ab5d5c9bceb0ace3a06538b5e40be96094c4c9a72.
//
// Solidity: event DistributeFee(address indexed receiver, string fee)
func (_RelayerModule *RelayerModuleFilterer) FilterDistributeFee(opts *bind.FilterOpts, receiver []common.Address) (*RelayerModuleDistributeFeeIterator, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "DistributeFee", receiverRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleDistributeFeeIterator{contract: _RelayerModule.contract, event: "DistributeFee", logs: logs, sub: sub}, nil
}

// WatchDistributeFee is a free log subscription operation binding the contract event 0x67e2bceb7881996b4bbddf9ab5d5c9bceb0ace3a06538b5e40be96094c4c9a72.
//
// Solidity: event DistributeFee(address indexed receiver, string fee)
func (_RelayerModule *RelayerModuleFilterer) WatchDistributeFee(opts *bind.WatchOpts, sink chan<- *RelayerModuleDistributeFee, receiver []common.Address) (event.Subscription, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "DistributeFee", receiverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleDistributeFee)
				if err := _RelayerModule.contract.UnpackLog(event, "DistributeFee", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDistributeFee is a log parse operation binding the contract event 0x67e2bceb7881996b4bbddf9ab5d5c9bceb0ace3a06538b5e40be96094c4c9a72.
//
// Solidity: event DistributeFee(address indexed receiver, string fee)
func (_RelayerModule *RelayerModuleFilterer) ParseDistributeFee(log types.Log) (*RelayerModuleDistributeFee, error) {
	event := new(RelayerModuleDistributeFee)
	if err := _RelayerModule.contract.UnpackLog(event, "DistributeFee", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleFungibleTokenPacketIterator is returned from FilterFungibleTokenPacket and is used to iterate over the raw logs and unpacked data for FungibleTokenPacket events raised by the RelayerModule contract.
type RelayerModuleFungibleTokenPacketIterator struct {
	Event *RelayerModuleFungibleTokenPacket // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleFungibleTokenPacketIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleFungibleTokenPacket)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleFungibleTokenPacket)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleFungibleTokenPacketIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleFungibleTokenPacketIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleFungibleTokenPacket represents a FungibleTokenPacket event raised by the RelayerModule contract.
type RelayerModuleFungibleTokenPacket struct {
	Receiver common.Address
	Sender   common.Address
	Denom    string
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterFungibleTokenPacket is a free log retrieval operation binding the contract event 0xe0fdee6007dd2fb6acfd338163a4260f0abf107fc184f28b75c5b2c1be55f573.
//
// Solidity: event FungibleTokenPacket(address indexed receiver, address indexed sender, string denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) FilterFungibleTokenPacket(opts *bind.FilterOpts, receiver []common.Address, sender []common.Address) (*RelayerModuleFungibleTokenPacketIterator, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "FungibleTokenPacket", receiverRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleFungibleTokenPacketIterator{contract: _RelayerModule.contract, event: "FungibleTokenPacket", logs: logs, sub: sub}, nil
}

// WatchFungibleTokenPacket is a free log subscription operation binding the contract event 0xe0fdee6007dd2fb6acfd338163a4260f0abf107fc184f28b75c5b2c1be55f573.
//
// Solidity: event FungibleTokenPacket(address indexed receiver, address indexed sender, string denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) WatchFungibleTokenPacket(opts *bind.WatchOpts, sink chan<- *RelayerModuleFungibleTokenPacket, receiver []common.Address, sender []common.Address) (event.Subscription, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "FungibleTokenPacket", receiverRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleFungibleTokenPacket)
				if err := _RelayerModule.contract.UnpackLog(event, "FungibleTokenPacket", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFungibleTokenPacket is a log parse operation binding the contract event 0xe0fdee6007dd2fb6acfd338163a4260f0abf107fc184f28b75c5b2c1be55f573.
//
// Solidity: event FungibleTokenPacket(address indexed receiver, address indexed sender, string denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) ParseFungibleTokenPacket(log types.Log) (*RelayerModuleFungibleTokenPacket, error) {
	event := new(RelayerModuleFungibleTokenPacket)
	if err := _RelayerModule.contract.UnpackLog(event, "FungibleTokenPacket", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleIbcTransferIterator is returned from FilterIbcTransfer and is used to iterate over the raw logs and unpacked data for IbcTransfer events raised by the RelayerModule contract.
type RelayerModuleIbcTransferIterator struct {
	Event *RelayerModuleIbcTransfer // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleIbcTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleIbcTransfer)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleIbcTransfer)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleIbcTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleIbcTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleIbcTransfer represents a IbcTransfer event raised by the RelayerModule contract.
type RelayerModuleIbcTransfer struct {
	Sender   common.Address
	Receiver common.Address
	Denom    string
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterIbcTransfer is a free log retrieval operation binding the contract event 0x3015d0e0dd0cda983ea101408cb97c173ec3a038ee6f439cc3c7532c52057c0c.
//
// Solidity: event IbcTransfer(address indexed sender, address indexed receiver, string denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) FilterIbcTransfer(opts *bind.FilterOpts, sender []common.Address, receiver []common.Address) (*RelayerModuleIbcTransferIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "IbcTransfer", senderRule, receiverRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleIbcTransferIterator{contract: _RelayerModule.contract, event: "IbcTransfer", logs: logs, sub: sub}, nil
}

// WatchIbcTransfer is a free log subscription operation binding the contract event 0x3015d0e0dd0cda983ea101408cb97c173ec3a038ee6f439cc3c7532c52057c0c.
//
// Solidity: event IbcTransfer(address indexed sender, address indexed receiver, string denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) WatchIbcTransfer(opts *bind.WatchOpts, sink chan<- *RelayerModuleIbcTransfer, sender []common.Address, receiver []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "IbcTransfer", senderRule, receiverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleIbcTransfer)
				if err := _RelayerModule.contract.UnpackLog(event, "IbcTransfer", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseIbcTransfer is a log parse operation binding the contract event 0x3015d0e0dd0cda983ea101408cb97c173ec3a038ee6f439cc3c7532c52057c0c.
//
// Solidity: event IbcTransfer(address indexed sender, address indexed receiver, string denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) ParseIbcTransfer(log types.Log) (*RelayerModuleIbcTransfer, error) {
	event := new(RelayerModuleIbcTransfer)
	if err := _RelayerModule.contract.UnpackLog(event, "IbcTransfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleRecvPacketIterator is returned from FilterRecvPacket and is used to iterate over the raw logs and unpacked data for RecvPacket events raised by the RelayerModule contract.
type RelayerModuleRecvPacketIterator struct {
	Event *RelayerModuleRecvPacket // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleRecvPacketIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleRecvPacket)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleRecvPacket)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleRecvPacketIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleRecvPacketIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleRecvPacket represents a RecvPacket event raised by the RelayerModule contract.
type RelayerModuleRecvPacket struct {
	PacketSequence       *big.Int
	PacketSrcPort        common.Hash
	PacketSrcChannel     common.Hash
	PacketSrcPortInfo    string
	PacketSrcChannelInfo string
	PacketDstPort        string
	PacketDstChannel     string
	ConnectionId         string
	PacketDataHex        IRelayerModulePacketData
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterRecvPacket is a free log retrieval operation binding the contract event 0x4657bfadd5a5dfb224fe22b6d305862abb6b9f2b24b800c28d4ed86e477675c9.
//
// Solidity: event RecvPacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId, (address,string,(uint256,string)[]) packetDataHex)
func (_RelayerModule *RelayerModuleFilterer) FilterRecvPacket(opts *bind.FilterOpts, packetSequence []*big.Int, packetSrcPort []string, packetSrcChannel []string) (*RelayerModuleRecvPacketIterator, error) {

	var packetSequenceRule []interface{}
	for _, packetSequenceItem := range packetSequence {
		packetSequenceRule = append(packetSequenceRule, packetSequenceItem)
	}
	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "RecvPacket", packetSequenceRule, packetSrcPortRule, packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleRecvPacketIterator{contract: _RelayerModule.contract, event: "RecvPacket", logs: logs, sub: sub}, nil
}

// WatchRecvPacket is a free log subscription operation binding the contract event 0x4657bfadd5a5dfb224fe22b6d305862abb6b9f2b24b800c28d4ed86e477675c9.
//
// Solidity: event RecvPacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId, (address,string,(uint256,string)[]) packetDataHex)
func (_RelayerModule *RelayerModuleFilterer) WatchRecvPacket(opts *bind.WatchOpts, sink chan<- *RelayerModuleRecvPacket, packetSequence []*big.Int, packetSrcPort []string, packetSrcChannel []string) (event.Subscription, error) {

	var packetSequenceRule []interface{}
	for _, packetSequenceItem := range packetSequence {
		packetSequenceRule = append(packetSequenceRule, packetSequenceItem)
	}
	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "RecvPacket", packetSequenceRule, packetSrcPortRule, packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleRecvPacket)
				if err := _RelayerModule.contract.UnpackLog(event, "RecvPacket", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRecvPacket is a log parse operation binding the contract event 0x4657bfadd5a5dfb224fe22b6d305862abb6b9f2b24b800c28d4ed86e477675c9.
//
// Solidity: event RecvPacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId, (address,string,(uint256,string)[]) packetDataHex)
func (_RelayerModule *RelayerModuleFilterer) ParseRecvPacket(log types.Log) (*RelayerModuleRecvPacket, error) {
	event := new(RelayerModuleRecvPacket)
	if err := _RelayerModule.contract.UnpackLog(event, "RecvPacket", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleTimeoutIterator is returned from FilterTimeout and is used to iterate over the raw logs and unpacked data for Timeout events raised by the RelayerModule contract.
type RelayerModuleTimeoutIterator struct {
	Event *RelayerModuleTimeout // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleTimeoutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleTimeout)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleTimeout)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleTimeoutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleTimeoutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleTimeout represents a Timeout event raised by the RelayerModule contract.
type RelayerModuleTimeout struct {
	RefundReceiver common.Address
	RefundDenom    string
	RefundAmount   string
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterTimeout is a free log retrieval operation binding the contract event 0x2048e680af69959f50b9e0ae0b475b973d0c7b8e21b2d4c026beeb9a626dedd7.
//
// Solidity: event Timeout(address indexed refundReceiver, string refundDenom, string refundAmount)
func (_RelayerModule *RelayerModuleFilterer) FilterTimeout(opts *bind.FilterOpts, refundReceiver []common.Address) (*RelayerModuleTimeoutIterator, error) {

	var refundReceiverRule []interface{}
	for _, refundReceiverItem := range refundReceiver {
		refundReceiverRule = append(refundReceiverRule, refundReceiverItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "Timeout", refundReceiverRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleTimeoutIterator{contract: _RelayerModule.contract, event: "Timeout", logs: logs, sub: sub}, nil
}

// WatchTimeout is a free log subscription operation binding the contract event 0x2048e680af69959f50b9e0ae0b475b973d0c7b8e21b2d4c026beeb9a626dedd7.
//
// Solidity: event Timeout(address indexed refundReceiver, string refundDenom, string refundAmount)
func (_RelayerModule *RelayerModuleFilterer) WatchTimeout(opts *bind.WatchOpts, sink chan<- *RelayerModuleTimeout, refundReceiver []common.Address) (event.Subscription, error) {

	var refundReceiverRule []interface{}
	for _, refundReceiverItem := range refundReceiver {
		refundReceiverRule = append(refundReceiverRule, refundReceiverItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "Timeout", refundReceiverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleTimeout)
				if err := _RelayerModule.contract.UnpackLog(event, "Timeout", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTimeout is a log parse operation binding the contract event 0x2048e680af69959f50b9e0ae0b475b973d0c7b8e21b2d4c026beeb9a626dedd7.
//
// Solidity: event Timeout(address indexed refundReceiver, string refundDenom, string refundAmount)
func (_RelayerModule *RelayerModuleFilterer) ParseTimeout(log types.Log) (*RelayerModuleTimeout, error) {
	event := new(RelayerModuleTimeout)
	if err := _RelayerModule.contract.UnpackLog(event, "Timeout", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleTimeoutPacketIterator is returned from FilterTimeoutPacket and is used to iterate over the raw logs and unpacked data for TimeoutPacket events raised by the RelayerModule contract.
type RelayerModuleTimeoutPacketIterator struct {
	Event *RelayerModuleTimeoutPacket // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleTimeoutPacketIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleTimeoutPacket)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleTimeoutPacket)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleTimeoutPacketIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleTimeoutPacketIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleTimeoutPacket represents a TimeoutPacket event raised by the RelayerModule contract.
type RelayerModuleTimeoutPacket struct {
	PacketSequence       *big.Int
	PacketSrcPort        common.Hash
	PacketSrcChannel     common.Hash
	PacketSrcPortInfo    string
	PacketSrcChannelInfo string
	PacketDstPort        string
	PacketDstChannel     string
	ConnectionId         string
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterTimeoutPacket is a free log retrieval operation binding the contract event 0x5d98a0d501e3e6d0cd441fe10aa3c19d72cd8cdf6402c6f2ee0e02e0d1cd6621.
//
// Solidity: event TimeoutPacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterTimeoutPacket(opts *bind.FilterOpts, packetSequence []*big.Int, packetSrcPort []string, packetSrcChannel []string) (*RelayerModuleTimeoutPacketIterator, error) {

	var packetSequenceRule []interface{}
	for _, packetSequenceItem := range packetSequence {
		packetSequenceRule = append(packetSequenceRule, packetSequenceItem)
	}
	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "TimeoutPacket", packetSequenceRule, packetSrcPortRule, packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleTimeoutPacketIterator{contract: _RelayerModule.contract, event: "TimeoutPacket", logs: logs, sub: sub}, nil
}

// WatchTimeoutPacket is a free log subscription operation binding the contract event 0x5d98a0d501e3e6d0cd441fe10aa3c19d72cd8cdf6402c6f2ee0e02e0d1cd6621.
//
// Solidity: event TimeoutPacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchTimeoutPacket(opts *bind.WatchOpts, sink chan<- *RelayerModuleTimeoutPacket, packetSequence []*big.Int, packetSrcPort []string, packetSrcChannel []string) (event.Subscription, error) {

	var packetSequenceRule []interface{}
	for _, packetSequenceItem := range packetSequence {
		packetSequenceRule = append(packetSequenceRule, packetSequenceItem)
	}
	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "TimeoutPacket", packetSequenceRule, packetSrcPortRule, packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleTimeoutPacket)
				if err := _RelayerModule.contract.UnpackLog(event, "TimeoutPacket", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTimeoutPacket is a log parse operation binding the contract event 0x5d98a0d501e3e6d0cd441fe10aa3c19d72cd8cdf6402c6f2ee0e02e0d1cd6621.
//
// Solidity: event TimeoutPacket(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseTimeoutPacket(log types.Log) (*RelayerModuleTimeoutPacket, error) {
	event := new(RelayerModuleTimeoutPacket)
	if err := _RelayerModule.contract.UnpackLog(event, "TimeoutPacket", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the RelayerModule contract.
type RelayerModuleTransferIterator struct {
	Event *RelayerModuleTransfer // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleTransfer)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleTransfer)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleTransfer represents a Transfer event raised by the RelayerModule contract.
type RelayerModuleTransfer struct {
	Recipient common.Address
	Sender    common.Address
	Amount    []CosmosCoin
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xbd3ee935be524866e9a469dd0f1cf61bf7f85eb70600ec7339433f4f2e8f44a6.
//
// Solidity: event Transfer(address indexed recipient, address indexed sender, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) FilterTransfer(opts *bind.FilterOpts, recipient []common.Address, sender []common.Address) (*RelayerModuleTransferIterator, error) {

	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "Transfer", recipientRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleTransferIterator{contract: _RelayerModule.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xbd3ee935be524866e9a469dd0f1cf61bf7f85eb70600ec7339433f4f2e8f44a6.
//
// Solidity: event Transfer(address indexed recipient, address indexed sender, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *RelayerModuleTransfer, recipient []common.Address, sender []common.Address) (event.Subscription, error) {

	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "Transfer", recipientRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleTransfer)
				if err := _RelayerModule.contract.UnpackLog(event, "Transfer", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTransfer is a log parse operation binding the contract event 0xbd3ee935be524866e9a469dd0f1cf61bf7f85eb70600ec7339433f4f2e8f44a6.
//
// Solidity: event Transfer(address indexed recipient, address indexed sender, (uint256,string)[] amount)
func (_RelayerModule *RelayerModuleFilterer) ParseTransfer(log types.Log) (*RelayerModuleTransfer, error) {
	event := new(RelayerModuleTransfer)
	if err := _RelayerModule.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleWriteAcknowledgementIterator is returned from FilterWriteAcknowledgement and is used to iterate over the raw logs and unpacked data for WriteAcknowledgement events raised by the RelayerModule contract.
type RelayerModuleWriteAcknowledgementIterator struct {
	Event *RelayerModuleWriteAcknowledgement // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayerModuleWriteAcknowledgementIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleWriteAcknowledgement)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayerModuleWriteAcknowledgement)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayerModuleWriteAcknowledgementIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleWriteAcknowledgementIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleWriteAcknowledgement represents a WriteAcknowledgement event raised by the RelayerModule contract.
type RelayerModuleWriteAcknowledgement struct {
	PacketSequence       *big.Int
	PacketSrcPort        common.Hash
	PacketSrcChannel     common.Hash
	PacketSrcPortInfo    string
	PacketSrcChannelInfo string
	PacketDstPort        string
	PacketDstChannel     string
	ConnectionId         string
	PacketDataHex        IRelayerModulePacketData
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterWriteAcknowledgement is a free log retrieval operation binding the contract event 0xee91ccc0582e9dc417ef1f48c2256e2a61357850c0932c080062c486143f99d4.
//
// Solidity: event WriteAcknowledgement(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId, (address,string,(uint256,string)[]) packetDataHex)
func (_RelayerModule *RelayerModuleFilterer) FilterWriteAcknowledgement(opts *bind.FilterOpts, packetSequence []*big.Int, packetSrcPort []string, packetSrcChannel []string) (*RelayerModuleWriteAcknowledgementIterator, error) {

	var packetSequenceRule []interface{}
	for _, packetSequenceItem := range packetSequence {
		packetSequenceRule = append(packetSequenceRule, packetSequenceItem)
	}
	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "WriteAcknowledgement", packetSequenceRule, packetSrcPortRule, packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleWriteAcknowledgementIterator{contract: _RelayerModule.contract, event: "WriteAcknowledgement", logs: logs, sub: sub}, nil
}

// WatchWriteAcknowledgement is a free log subscription operation binding the contract event 0xee91ccc0582e9dc417ef1f48c2256e2a61357850c0932c080062c486143f99d4.
//
// Solidity: event WriteAcknowledgement(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId, (address,string,(uint256,string)[]) packetDataHex)
func (_RelayerModule *RelayerModuleFilterer) WatchWriteAcknowledgement(opts *bind.WatchOpts, sink chan<- *RelayerModuleWriteAcknowledgement, packetSequence []*big.Int, packetSrcPort []string, packetSrcChannel []string) (event.Subscription, error) {

	var packetSequenceRule []interface{}
	for _, packetSequenceItem := range packetSequence {
		packetSequenceRule = append(packetSequenceRule, packetSequenceItem)
	}
	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "WriteAcknowledgement", packetSequenceRule, packetSrcPortRule, packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleWriteAcknowledgement)
				if err := _RelayerModule.contract.UnpackLog(event, "WriteAcknowledgement", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseWriteAcknowledgement is a log parse operation binding the contract event 0xee91ccc0582e9dc417ef1f48c2256e2a61357850c0932c080062c486143f99d4.
//
// Solidity: event WriteAcknowledgement(uint256 indexed packetSequence, string indexed packetSrcPort, string indexed packetSrcChannel, string packetSrcPortInfo, string packetSrcChannelInfo, string packetDstPort, string packetDstChannel, string connectionId, (address,string,(uint256,string)[]) packetDataHex)
func (_RelayerModule *RelayerModuleFilterer) ParseWriteAcknowledgement(log types.Log) (*RelayerModuleWriteAcknowledgement, error) {
	event := new(RelayerModuleWriteAcknowledgement)
	if err := _RelayerModule.contract.UnpackLog(event, "WriteAcknowledgement", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
