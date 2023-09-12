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
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcPort\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcChannel\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetDstPort\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetChannelOrdering\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetConnection\",\"type\":\"string\"}],\"name\":\"AcknowledgePacket\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"burner\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"Burn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"portId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"channelId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyPortId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyChannelId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"}],\"name\":\"ChannelCloseConfirm\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"portId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"channelId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyPortId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyChannelId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"}],\"name\":\"ChannelCloseInit\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"ChannelClosed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"portId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"channelId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyPortId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyChannelId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"}],\"name\":\"ChannelOpenAck\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"portId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"channelId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyPortId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyChannelId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"}],\"name\":\"ChannelOpenConfirm\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"portId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"channelId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyPortId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyChannelId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"version\",\"type\":\"string\"}],\"name\":\"ChannelOpenInit\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"portId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"channelId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyPortId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyChannelId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"version\",\"type\":\"string\"}],\"name\":\"ChannelOpenTry\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"CoinReceived\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"CoinSpent\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"minter\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"Coinbase\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyClientId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyConnectionId\",\"type\":\"string\"}],\"name\":\"ConnectionOpenAck\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyClientId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyConnectionId\",\"type\":\"string\"}],\"name\":\"ConnectionOpenConfirm\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyClientId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyConnectionId\",\"type\":\"string\"}],\"name\":\"ConnectionOpenInit\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"connectionId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"counterpartyClientId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"counterpartyConnectionId\",\"type\":\"string\"}],\"name\":\"ConnectionOpenTry\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientType\",\"type\":\"string\"}],\"name\":\"CreateClient\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"name\":\"DenominationTrace\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"fee\",\"type\":\"string\"}],\"name\":\"DistributeFee\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"FungibleTokenPacket\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"IbcTransfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Message\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"sender\",\"type\":\"string\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"indexed\":false,\"internalType\":\"structIRelayerModule.PacketData\",\"name\":\"packetData\",\"type\":\"tuple\"}],\"name\":\"RecvPacket\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"subjectId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientType\",\"type\":\"string\"}],\"name\":\"SubmitMisbehaviour\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"refundReceiver\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"refundDenom\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Timeout\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcPort\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcChannel\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetDstPort\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetDstChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetChannelOrdering\",\"type\":\"string\"}],\"name\":\"TimeoutPacket\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"indexed\":false,\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientType\",\"type\":\"string\"}],\"name\":\"UpdateClient\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientId\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"clientType\",\"type\":\"string\"}],\"name\":\"UpgradeClient\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"packetConnection\",\"type\":\"string\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"sender\",\"type\":\"string\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"internalType\":\"structCosmos.Coin[]\",\"name\":\"amount\",\"type\":\"tuple[]\"}],\"indexed\":false,\"internalType\":\"structIRelayerModule.PacketData\",\"name\":\"packetData\",\"type\":\"tuple\"}],\"name\":\"WriteAcknowledgement\",\"type\":\"event\"}]",
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
	parsed, err := abi.JSON(strings.NewReader(RelayerModuleABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
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
	PacketSrcPort         common.Hash
	PacketSrcChannel      common.Hash
	PacketDstPort         common.Hash
	PacketDstChannel      string
	PacketChannelOrdering string
	PacketConnection      string
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterAcknowledgePacket is a free log retrieval operation binding the contract event 0xc7b594c06c08a3e531587ce7aea85dd2d77aa812e153185114587f284481ee2d.
//
// Solidity: event AcknowledgePacket(string indexed packetSrcPort, string indexed packetSrcChannel, string indexed packetDstPort, string packetDstChannel, string packetChannelOrdering, string packetConnection)
func (_RelayerModule *RelayerModuleFilterer) FilterAcknowledgePacket(opts *bind.FilterOpts, packetSrcPort []string, packetSrcChannel []string, packetDstPort []string) (*RelayerModuleAcknowledgePacketIterator, error) {

	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}
	var packetDstPortRule []interface{}
	for _, packetDstPortItem := range packetDstPort {
		packetDstPortRule = append(packetDstPortRule, packetDstPortItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "AcknowledgePacket", packetSrcPortRule, packetSrcChannelRule, packetDstPortRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleAcknowledgePacketIterator{contract: _RelayerModule.contract, event: "AcknowledgePacket", logs: logs, sub: sub}, nil
}

// WatchAcknowledgePacket is a free log subscription operation binding the contract event 0xc7b594c06c08a3e531587ce7aea85dd2d77aa812e153185114587f284481ee2d.
//
// Solidity: event AcknowledgePacket(string indexed packetSrcPort, string indexed packetSrcChannel, string indexed packetDstPort, string packetDstChannel, string packetChannelOrdering, string packetConnection)
func (_RelayerModule *RelayerModuleFilterer) WatchAcknowledgePacket(opts *bind.WatchOpts, sink chan<- *RelayerModuleAcknowledgePacket, packetSrcPort []string, packetSrcChannel []string, packetDstPort []string) (event.Subscription, error) {

	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}
	var packetDstPortRule []interface{}
	for _, packetDstPortItem := range packetDstPort {
		packetDstPortRule = append(packetDstPortRule, packetDstPortItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "AcknowledgePacket", packetSrcPortRule, packetSrcChannelRule, packetDstPortRule)
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

// ParseAcknowledgePacket is a log parse operation binding the contract event 0xc7b594c06c08a3e531587ce7aea85dd2d77aa812e153185114587f284481ee2d.
//
// Solidity: event AcknowledgePacket(string indexed packetSrcPort, string indexed packetSrcChannel, string indexed packetDstPort, string packetDstChannel, string packetChannelOrdering, string packetConnection)
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

// RelayerModuleChannelCloseConfirmIterator is returned from FilterChannelCloseConfirm and is used to iterate over the raw logs and unpacked data for ChannelCloseConfirm events raised by the RelayerModule contract.
type RelayerModuleChannelCloseConfirmIterator struct {
	Event *RelayerModuleChannelCloseConfirm // Event containing the contract specifics and raw log

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
func (it *RelayerModuleChannelCloseConfirmIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleChannelCloseConfirm)
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
		it.Event = new(RelayerModuleChannelCloseConfirm)
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
func (it *RelayerModuleChannelCloseConfirmIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleChannelCloseConfirmIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleChannelCloseConfirm represents a ChannelCloseConfirm event raised by the RelayerModule contract.
type RelayerModuleChannelCloseConfirm struct {
	PortId                common.Hash
	ChannelId             common.Hash
	CounterpartyPortId    common.Hash
	CounterpartyChannelId string
	ConnectionId          string
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterChannelCloseConfirm is a free log retrieval operation binding the contract event 0x1d27827947f32db531c2d0a11a83e392e9391cf32071f1716bc53c3df605b637.
//
// Solidity: event ChannelCloseConfirm(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterChannelCloseConfirm(opts *bind.FilterOpts, portId []string, channelId []string, counterpartyPortId []string) (*RelayerModuleChannelCloseConfirmIterator, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ChannelCloseConfirm", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleChannelCloseConfirmIterator{contract: _RelayerModule.contract, event: "ChannelCloseConfirm", logs: logs, sub: sub}, nil
}

// WatchChannelCloseConfirm is a free log subscription operation binding the contract event 0x1d27827947f32db531c2d0a11a83e392e9391cf32071f1716bc53c3df605b637.
//
// Solidity: event ChannelCloseConfirm(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchChannelCloseConfirm(opts *bind.WatchOpts, sink chan<- *RelayerModuleChannelCloseConfirm, portId []string, channelId []string, counterpartyPortId []string) (event.Subscription, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ChannelCloseConfirm", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleChannelCloseConfirm)
				if err := _RelayerModule.contract.UnpackLog(event, "ChannelCloseConfirm", log); err != nil {
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

// ParseChannelCloseConfirm is a log parse operation binding the contract event 0x1d27827947f32db531c2d0a11a83e392e9391cf32071f1716bc53c3df605b637.
//
// Solidity: event ChannelCloseConfirm(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseChannelCloseConfirm(log types.Log) (*RelayerModuleChannelCloseConfirm, error) {
	event := new(RelayerModuleChannelCloseConfirm)
	if err := _RelayerModule.contract.UnpackLog(event, "ChannelCloseConfirm", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleChannelCloseInitIterator is returned from FilterChannelCloseInit and is used to iterate over the raw logs and unpacked data for ChannelCloseInit events raised by the RelayerModule contract.
type RelayerModuleChannelCloseInitIterator struct {
	Event *RelayerModuleChannelCloseInit // Event containing the contract specifics and raw log

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
func (it *RelayerModuleChannelCloseInitIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleChannelCloseInit)
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
		it.Event = new(RelayerModuleChannelCloseInit)
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
func (it *RelayerModuleChannelCloseInitIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleChannelCloseInitIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleChannelCloseInit represents a ChannelCloseInit event raised by the RelayerModule contract.
type RelayerModuleChannelCloseInit struct {
	PortId                common.Hash
	ChannelId             common.Hash
	CounterpartyPortId    common.Hash
	CounterpartyChannelId string
	ConnectionId          string
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterChannelCloseInit is a free log retrieval operation binding the contract event 0x645976448b76cf17132a7f0f96d505a70aa349bc7753973035352feb57901375.
//
// Solidity: event ChannelCloseInit(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterChannelCloseInit(opts *bind.FilterOpts, portId []string, channelId []string, counterpartyPortId []string) (*RelayerModuleChannelCloseInitIterator, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ChannelCloseInit", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleChannelCloseInitIterator{contract: _RelayerModule.contract, event: "ChannelCloseInit", logs: logs, sub: sub}, nil
}

// WatchChannelCloseInit is a free log subscription operation binding the contract event 0x645976448b76cf17132a7f0f96d505a70aa349bc7753973035352feb57901375.
//
// Solidity: event ChannelCloseInit(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchChannelCloseInit(opts *bind.WatchOpts, sink chan<- *RelayerModuleChannelCloseInit, portId []string, channelId []string, counterpartyPortId []string) (event.Subscription, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ChannelCloseInit", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleChannelCloseInit)
				if err := _RelayerModule.contract.UnpackLog(event, "ChannelCloseInit", log); err != nil {
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

// ParseChannelCloseInit is a log parse operation binding the contract event 0x645976448b76cf17132a7f0f96d505a70aa349bc7753973035352feb57901375.
//
// Solidity: event ChannelCloseInit(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseChannelCloseInit(log types.Log) (*RelayerModuleChannelCloseInit, error) {
	event := new(RelayerModuleChannelCloseInit)
	if err := _RelayerModule.contract.UnpackLog(event, "ChannelCloseInit", log); err != nil {
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

// RelayerModuleChannelOpenAckIterator is returned from FilterChannelOpenAck and is used to iterate over the raw logs and unpacked data for ChannelOpenAck events raised by the RelayerModule contract.
type RelayerModuleChannelOpenAckIterator struct {
	Event *RelayerModuleChannelOpenAck // Event containing the contract specifics and raw log

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
func (it *RelayerModuleChannelOpenAckIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleChannelOpenAck)
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
		it.Event = new(RelayerModuleChannelOpenAck)
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
func (it *RelayerModuleChannelOpenAckIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleChannelOpenAckIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleChannelOpenAck represents a ChannelOpenAck event raised by the RelayerModule contract.
type RelayerModuleChannelOpenAck struct {
	PortId                common.Hash
	ChannelId             common.Hash
	CounterpartyPortId    common.Hash
	CounterpartyChannelId string
	ConnectionId          string
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterChannelOpenAck is a free log retrieval operation binding the contract event 0xe9342577bf02f748ba783edd9094f8e93b2ef7face9bc7478d7b30358ddeef6f.
//
// Solidity: event ChannelOpenAck(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterChannelOpenAck(opts *bind.FilterOpts, portId []string, channelId []string, counterpartyPortId []string) (*RelayerModuleChannelOpenAckIterator, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ChannelOpenAck", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleChannelOpenAckIterator{contract: _RelayerModule.contract, event: "ChannelOpenAck", logs: logs, sub: sub}, nil
}

// WatchChannelOpenAck is a free log subscription operation binding the contract event 0xe9342577bf02f748ba783edd9094f8e93b2ef7face9bc7478d7b30358ddeef6f.
//
// Solidity: event ChannelOpenAck(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchChannelOpenAck(opts *bind.WatchOpts, sink chan<- *RelayerModuleChannelOpenAck, portId []string, channelId []string, counterpartyPortId []string) (event.Subscription, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ChannelOpenAck", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleChannelOpenAck)
				if err := _RelayerModule.contract.UnpackLog(event, "ChannelOpenAck", log); err != nil {
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

// ParseChannelOpenAck is a log parse operation binding the contract event 0xe9342577bf02f748ba783edd9094f8e93b2ef7face9bc7478d7b30358ddeef6f.
//
// Solidity: event ChannelOpenAck(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseChannelOpenAck(log types.Log) (*RelayerModuleChannelOpenAck, error) {
	event := new(RelayerModuleChannelOpenAck)
	if err := _RelayerModule.contract.UnpackLog(event, "ChannelOpenAck", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleChannelOpenConfirmIterator is returned from FilterChannelOpenConfirm and is used to iterate over the raw logs and unpacked data for ChannelOpenConfirm events raised by the RelayerModule contract.
type RelayerModuleChannelOpenConfirmIterator struct {
	Event *RelayerModuleChannelOpenConfirm // Event containing the contract specifics and raw log

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
func (it *RelayerModuleChannelOpenConfirmIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleChannelOpenConfirm)
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
		it.Event = new(RelayerModuleChannelOpenConfirm)
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
func (it *RelayerModuleChannelOpenConfirmIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleChannelOpenConfirmIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleChannelOpenConfirm represents a ChannelOpenConfirm event raised by the RelayerModule contract.
type RelayerModuleChannelOpenConfirm struct {
	PortId                common.Hash
	ChannelId             common.Hash
	CounterpartyPortId    common.Hash
	CounterpartyChannelId string
	ConnectionId          string
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterChannelOpenConfirm is a free log retrieval operation binding the contract event 0xcccb79544f2a910ecd04c3bd96f870be6f5c74e0d00c18443c25ecf7b9800918.
//
// Solidity: event ChannelOpenConfirm(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterChannelOpenConfirm(opts *bind.FilterOpts, portId []string, channelId []string, counterpartyPortId []string) (*RelayerModuleChannelOpenConfirmIterator, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ChannelOpenConfirm", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleChannelOpenConfirmIterator{contract: _RelayerModule.contract, event: "ChannelOpenConfirm", logs: logs, sub: sub}, nil
}

// WatchChannelOpenConfirm is a free log subscription operation binding the contract event 0xcccb79544f2a910ecd04c3bd96f870be6f5c74e0d00c18443c25ecf7b9800918.
//
// Solidity: event ChannelOpenConfirm(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchChannelOpenConfirm(opts *bind.WatchOpts, sink chan<- *RelayerModuleChannelOpenConfirm, portId []string, channelId []string, counterpartyPortId []string) (event.Subscription, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ChannelOpenConfirm", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleChannelOpenConfirm)
				if err := _RelayerModule.contract.UnpackLog(event, "ChannelOpenConfirm", log); err != nil {
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

// ParseChannelOpenConfirm is a log parse operation binding the contract event 0xcccb79544f2a910ecd04c3bd96f870be6f5c74e0d00c18443c25ecf7b9800918.
//
// Solidity: event ChannelOpenConfirm(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseChannelOpenConfirm(log types.Log) (*RelayerModuleChannelOpenConfirm, error) {
	event := new(RelayerModuleChannelOpenConfirm)
	if err := _RelayerModule.contract.UnpackLog(event, "ChannelOpenConfirm", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleChannelOpenInitIterator is returned from FilterChannelOpenInit and is used to iterate over the raw logs and unpacked data for ChannelOpenInit events raised by the RelayerModule contract.
type RelayerModuleChannelOpenInitIterator struct {
	Event *RelayerModuleChannelOpenInit // Event containing the contract specifics and raw log

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
func (it *RelayerModuleChannelOpenInitIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleChannelOpenInit)
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
		it.Event = new(RelayerModuleChannelOpenInit)
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
func (it *RelayerModuleChannelOpenInitIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleChannelOpenInitIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleChannelOpenInit represents a ChannelOpenInit event raised by the RelayerModule contract.
type RelayerModuleChannelOpenInit struct {
	PortId                common.Hash
	ChannelId             common.Hash
	CounterpartyPortId    common.Hash
	CounterpartyChannelId string
	ConnectionId          string
	Version               string
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterChannelOpenInit is a free log retrieval operation binding the contract event 0xf80048ae930ff368af371496b4e42c025aabac6437dc6cac31865d3c7af74500.
//
// Solidity: event ChannelOpenInit(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId, string version)
func (_RelayerModule *RelayerModuleFilterer) FilterChannelOpenInit(opts *bind.FilterOpts, portId []string, channelId []string, counterpartyPortId []string) (*RelayerModuleChannelOpenInitIterator, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ChannelOpenInit", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleChannelOpenInitIterator{contract: _RelayerModule.contract, event: "ChannelOpenInit", logs: logs, sub: sub}, nil
}

// WatchChannelOpenInit is a free log subscription operation binding the contract event 0xf80048ae930ff368af371496b4e42c025aabac6437dc6cac31865d3c7af74500.
//
// Solidity: event ChannelOpenInit(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId, string version)
func (_RelayerModule *RelayerModuleFilterer) WatchChannelOpenInit(opts *bind.WatchOpts, sink chan<- *RelayerModuleChannelOpenInit, portId []string, channelId []string, counterpartyPortId []string) (event.Subscription, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ChannelOpenInit", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleChannelOpenInit)
				if err := _RelayerModule.contract.UnpackLog(event, "ChannelOpenInit", log); err != nil {
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

// ParseChannelOpenInit is a log parse operation binding the contract event 0xf80048ae930ff368af371496b4e42c025aabac6437dc6cac31865d3c7af74500.
//
// Solidity: event ChannelOpenInit(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId, string version)
func (_RelayerModule *RelayerModuleFilterer) ParseChannelOpenInit(log types.Log) (*RelayerModuleChannelOpenInit, error) {
	event := new(RelayerModuleChannelOpenInit)
	if err := _RelayerModule.contract.UnpackLog(event, "ChannelOpenInit", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleChannelOpenTryIterator is returned from FilterChannelOpenTry and is used to iterate over the raw logs and unpacked data for ChannelOpenTry events raised by the RelayerModule contract.
type RelayerModuleChannelOpenTryIterator struct {
	Event *RelayerModuleChannelOpenTry // Event containing the contract specifics and raw log

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
func (it *RelayerModuleChannelOpenTryIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleChannelOpenTry)
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
		it.Event = new(RelayerModuleChannelOpenTry)
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
func (it *RelayerModuleChannelOpenTryIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleChannelOpenTryIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleChannelOpenTry represents a ChannelOpenTry event raised by the RelayerModule contract.
type RelayerModuleChannelOpenTry struct {
	PortId                common.Hash
	ChannelId             common.Hash
	CounterpartyPortId    common.Hash
	CounterpartyChannelId string
	ConnectionId          string
	Version               string
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterChannelOpenTry is a free log retrieval operation binding the contract event 0x9c5a76e8bddb2e5c238e35b7ce7a850ad22a776479bfc8b4af5e88e073fa9c70.
//
// Solidity: event ChannelOpenTry(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId, string version)
func (_RelayerModule *RelayerModuleFilterer) FilterChannelOpenTry(opts *bind.FilterOpts, portId []string, channelId []string, counterpartyPortId []string) (*RelayerModuleChannelOpenTryIterator, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ChannelOpenTry", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleChannelOpenTryIterator{contract: _RelayerModule.contract, event: "ChannelOpenTry", logs: logs, sub: sub}, nil
}

// WatchChannelOpenTry is a free log subscription operation binding the contract event 0x9c5a76e8bddb2e5c238e35b7ce7a850ad22a776479bfc8b4af5e88e073fa9c70.
//
// Solidity: event ChannelOpenTry(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId, string version)
func (_RelayerModule *RelayerModuleFilterer) WatchChannelOpenTry(opts *bind.WatchOpts, sink chan<- *RelayerModuleChannelOpenTry, portId []string, channelId []string, counterpartyPortId []string) (event.Subscription, error) {

	var portIdRule []interface{}
	for _, portIdItem := range portId {
		portIdRule = append(portIdRule, portIdItem)
	}
	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}
	var counterpartyPortIdRule []interface{}
	for _, counterpartyPortIdItem := range counterpartyPortId {
		counterpartyPortIdRule = append(counterpartyPortIdRule, counterpartyPortIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ChannelOpenTry", portIdRule, channelIdRule, counterpartyPortIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleChannelOpenTry)
				if err := _RelayerModule.contract.UnpackLog(event, "ChannelOpenTry", log); err != nil {
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

// ParseChannelOpenTry is a log parse operation binding the contract event 0x9c5a76e8bddb2e5c238e35b7ce7a850ad22a776479bfc8b4af5e88e073fa9c70.
//
// Solidity: event ChannelOpenTry(string indexed portId, string indexed channelId, string indexed counterpartyPortId, string counterpartyChannelId, string connectionId, string version)
func (_RelayerModule *RelayerModuleFilterer) ParseChannelOpenTry(log types.Log) (*RelayerModuleChannelOpenTry, error) {
	event := new(RelayerModuleChannelOpenTry)
	if err := _RelayerModule.contract.UnpackLog(event, "ChannelOpenTry", log); err != nil {
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

// RelayerModuleConnectionOpenAckIterator is returned from FilterConnectionOpenAck and is used to iterate over the raw logs and unpacked data for ConnectionOpenAck events raised by the RelayerModule contract.
type RelayerModuleConnectionOpenAckIterator struct {
	Event *RelayerModuleConnectionOpenAck // Event containing the contract specifics and raw log

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
func (it *RelayerModuleConnectionOpenAckIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleConnectionOpenAck)
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
		it.Event = new(RelayerModuleConnectionOpenAck)
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
func (it *RelayerModuleConnectionOpenAckIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleConnectionOpenAckIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleConnectionOpenAck represents a ConnectionOpenAck event raised by the RelayerModule contract.
type RelayerModuleConnectionOpenAck struct {
	ConnectionId             common.Hash
	ClientId                 common.Hash
	CounterpartyClientId     common.Hash
	CounterpartyConnectionId string
	Raw                      types.Log // Blockchain specific contextual infos
}

// FilterConnectionOpenAck is a free log retrieval operation binding the contract event 0xe7615b4ebffcb930061f901cc07ee67b4d32c8f9052141eb8bce2dec3f577fe1.
//
// Solidity: event ConnectionOpenAck(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterConnectionOpenAck(opts *bind.FilterOpts, connectionId []string, clientId []string, counterpartyClientId []string) (*RelayerModuleConnectionOpenAckIterator, error) {

	var connectionIdRule []interface{}
	for _, connectionIdItem := range connectionId {
		connectionIdRule = append(connectionIdRule, connectionIdItem)
	}
	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var counterpartyClientIdRule []interface{}
	for _, counterpartyClientIdItem := range counterpartyClientId {
		counterpartyClientIdRule = append(counterpartyClientIdRule, counterpartyClientIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ConnectionOpenAck", connectionIdRule, clientIdRule, counterpartyClientIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleConnectionOpenAckIterator{contract: _RelayerModule.contract, event: "ConnectionOpenAck", logs: logs, sub: sub}, nil
}

// WatchConnectionOpenAck is a free log subscription operation binding the contract event 0xe7615b4ebffcb930061f901cc07ee67b4d32c8f9052141eb8bce2dec3f577fe1.
//
// Solidity: event ConnectionOpenAck(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchConnectionOpenAck(opts *bind.WatchOpts, sink chan<- *RelayerModuleConnectionOpenAck, connectionId []string, clientId []string, counterpartyClientId []string) (event.Subscription, error) {

	var connectionIdRule []interface{}
	for _, connectionIdItem := range connectionId {
		connectionIdRule = append(connectionIdRule, connectionIdItem)
	}
	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var counterpartyClientIdRule []interface{}
	for _, counterpartyClientIdItem := range counterpartyClientId {
		counterpartyClientIdRule = append(counterpartyClientIdRule, counterpartyClientIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ConnectionOpenAck", connectionIdRule, clientIdRule, counterpartyClientIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleConnectionOpenAck)
				if err := _RelayerModule.contract.UnpackLog(event, "ConnectionOpenAck", log); err != nil {
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

// ParseConnectionOpenAck is a log parse operation binding the contract event 0xe7615b4ebffcb930061f901cc07ee67b4d32c8f9052141eb8bce2dec3f577fe1.
//
// Solidity: event ConnectionOpenAck(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseConnectionOpenAck(log types.Log) (*RelayerModuleConnectionOpenAck, error) {
	event := new(RelayerModuleConnectionOpenAck)
	if err := _RelayerModule.contract.UnpackLog(event, "ConnectionOpenAck", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleConnectionOpenConfirmIterator is returned from FilterConnectionOpenConfirm and is used to iterate over the raw logs and unpacked data for ConnectionOpenConfirm events raised by the RelayerModule contract.
type RelayerModuleConnectionOpenConfirmIterator struct {
	Event *RelayerModuleConnectionOpenConfirm // Event containing the contract specifics and raw log

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
func (it *RelayerModuleConnectionOpenConfirmIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleConnectionOpenConfirm)
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
		it.Event = new(RelayerModuleConnectionOpenConfirm)
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
func (it *RelayerModuleConnectionOpenConfirmIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleConnectionOpenConfirmIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleConnectionOpenConfirm represents a ConnectionOpenConfirm event raised by the RelayerModule contract.
type RelayerModuleConnectionOpenConfirm struct {
	ConnectionId             common.Hash
	ClientId                 common.Hash
	CounterpartyClientId     common.Hash
	CounterpartyConnectionId string
	Raw                      types.Log // Blockchain specific contextual infos
}

// FilterConnectionOpenConfirm is a free log retrieval operation binding the contract event 0x063c0e9664347d8013d3575d502050fd936d3b51035f056696a639523feaed6d.
//
// Solidity: event ConnectionOpenConfirm(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterConnectionOpenConfirm(opts *bind.FilterOpts, connectionId []string, clientId []string, counterpartyClientId []string) (*RelayerModuleConnectionOpenConfirmIterator, error) {

	var connectionIdRule []interface{}
	for _, connectionIdItem := range connectionId {
		connectionIdRule = append(connectionIdRule, connectionIdItem)
	}
	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var counterpartyClientIdRule []interface{}
	for _, counterpartyClientIdItem := range counterpartyClientId {
		counterpartyClientIdRule = append(counterpartyClientIdRule, counterpartyClientIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ConnectionOpenConfirm", connectionIdRule, clientIdRule, counterpartyClientIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleConnectionOpenConfirmIterator{contract: _RelayerModule.contract, event: "ConnectionOpenConfirm", logs: logs, sub: sub}, nil
}

// WatchConnectionOpenConfirm is a free log subscription operation binding the contract event 0x063c0e9664347d8013d3575d502050fd936d3b51035f056696a639523feaed6d.
//
// Solidity: event ConnectionOpenConfirm(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchConnectionOpenConfirm(opts *bind.WatchOpts, sink chan<- *RelayerModuleConnectionOpenConfirm, connectionId []string, clientId []string, counterpartyClientId []string) (event.Subscription, error) {

	var connectionIdRule []interface{}
	for _, connectionIdItem := range connectionId {
		connectionIdRule = append(connectionIdRule, connectionIdItem)
	}
	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var counterpartyClientIdRule []interface{}
	for _, counterpartyClientIdItem := range counterpartyClientId {
		counterpartyClientIdRule = append(counterpartyClientIdRule, counterpartyClientIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ConnectionOpenConfirm", connectionIdRule, clientIdRule, counterpartyClientIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleConnectionOpenConfirm)
				if err := _RelayerModule.contract.UnpackLog(event, "ConnectionOpenConfirm", log); err != nil {
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

// ParseConnectionOpenConfirm is a log parse operation binding the contract event 0x063c0e9664347d8013d3575d502050fd936d3b51035f056696a639523feaed6d.
//
// Solidity: event ConnectionOpenConfirm(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseConnectionOpenConfirm(log types.Log) (*RelayerModuleConnectionOpenConfirm, error) {
	event := new(RelayerModuleConnectionOpenConfirm)
	if err := _RelayerModule.contract.UnpackLog(event, "ConnectionOpenConfirm", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleConnectionOpenInitIterator is returned from FilterConnectionOpenInit and is used to iterate over the raw logs and unpacked data for ConnectionOpenInit events raised by the RelayerModule contract.
type RelayerModuleConnectionOpenInitIterator struct {
	Event *RelayerModuleConnectionOpenInit // Event containing the contract specifics and raw log

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
func (it *RelayerModuleConnectionOpenInitIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleConnectionOpenInit)
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
		it.Event = new(RelayerModuleConnectionOpenInit)
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
func (it *RelayerModuleConnectionOpenInitIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleConnectionOpenInitIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleConnectionOpenInit represents a ConnectionOpenInit event raised by the RelayerModule contract.
type RelayerModuleConnectionOpenInit struct {
	ConnectionId             common.Hash
	ClientId                 common.Hash
	CounterpartyClientId     common.Hash
	CounterpartyConnectionId string
	Raw                      types.Log // Blockchain specific contextual infos
}

// FilterConnectionOpenInit is a free log retrieval operation binding the contract event 0x77fdc802b8b6331cb2c8d6bd52b9872dd22a59d21350263d1bf00b702a48f6e9.
//
// Solidity: event ConnectionOpenInit(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterConnectionOpenInit(opts *bind.FilterOpts, connectionId []string, clientId []string, counterpartyClientId []string) (*RelayerModuleConnectionOpenInitIterator, error) {

	var connectionIdRule []interface{}
	for _, connectionIdItem := range connectionId {
		connectionIdRule = append(connectionIdRule, connectionIdItem)
	}
	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var counterpartyClientIdRule []interface{}
	for _, counterpartyClientIdItem := range counterpartyClientId {
		counterpartyClientIdRule = append(counterpartyClientIdRule, counterpartyClientIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ConnectionOpenInit", connectionIdRule, clientIdRule, counterpartyClientIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleConnectionOpenInitIterator{contract: _RelayerModule.contract, event: "ConnectionOpenInit", logs: logs, sub: sub}, nil
}

// WatchConnectionOpenInit is a free log subscription operation binding the contract event 0x77fdc802b8b6331cb2c8d6bd52b9872dd22a59d21350263d1bf00b702a48f6e9.
//
// Solidity: event ConnectionOpenInit(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchConnectionOpenInit(opts *bind.WatchOpts, sink chan<- *RelayerModuleConnectionOpenInit, connectionId []string, clientId []string, counterpartyClientId []string) (event.Subscription, error) {

	var connectionIdRule []interface{}
	for _, connectionIdItem := range connectionId {
		connectionIdRule = append(connectionIdRule, connectionIdItem)
	}
	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var counterpartyClientIdRule []interface{}
	for _, counterpartyClientIdItem := range counterpartyClientId {
		counterpartyClientIdRule = append(counterpartyClientIdRule, counterpartyClientIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ConnectionOpenInit", connectionIdRule, clientIdRule, counterpartyClientIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleConnectionOpenInit)
				if err := _RelayerModule.contract.UnpackLog(event, "ConnectionOpenInit", log); err != nil {
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

// ParseConnectionOpenInit is a log parse operation binding the contract event 0x77fdc802b8b6331cb2c8d6bd52b9872dd22a59d21350263d1bf00b702a48f6e9.
//
// Solidity: event ConnectionOpenInit(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseConnectionOpenInit(log types.Log) (*RelayerModuleConnectionOpenInit, error) {
	event := new(RelayerModuleConnectionOpenInit)
	if err := _RelayerModule.contract.UnpackLog(event, "ConnectionOpenInit", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleConnectionOpenTryIterator is returned from FilterConnectionOpenTry and is used to iterate over the raw logs and unpacked data for ConnectionOpenTry events raised by the RelayerModule contract.
type RelayerModuleConnectionOpenTryIterator struct {
	Event *RelayerModuleConnectionOpenTry // Event containing the contract specifics and raw log

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
func (it *RelayerModuleConnectionOpenTryIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleConnectionOpenTry)
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
		it.Event = new(RelayerModuleConnectionOpenTry)
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
func (it *RelayerModuleConnectionOpenTryIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleConnectionOpenTryIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleConnectionOpenTry represents a ConnectionOpenTry event raised by the RelayerModule contract.
type RelayerModuleConnectionOpenTry struct {
	ConnectionId             common.Hash
	ClientId                 common.Hash
	CounterpartyClientId     common.Hash
	CounterpartyConnectionId string
	Raw                      types.Log // Blockchain specific contextual infos
}

// FilterConnectionOpenTry is a free log retrieval operation binding the contract event 0xa616a9aa2c65e935abbd15b07a9b5ff6c9c48b06b460a39b0b8cfda2a985869f.
//
// Solidity: event ConnectionOpenTry(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) FilterConnectionOpenTry(opts *bind.FilterOpts, connectionId []string, clientId []string, counterpartyClientId []string) (*RelayerModuleConnectionOpenTryIterator, error) {

	var connectionIdRule []interface{}
	for _, connectionIdItem := range connectionId {
		connectionIdRule = append(connectionIdRule, connectionIdItem)
	}
	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var counterpartyClientIdRule []interface{}
	for _, counterpartyClientIdItem := range counterpartyClientId {
		counterpartyClientIdRule = append(counterpartyClientIdRule, counterpartyClientIdItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "ConnectionOpenTry", connectionIdRule, clientIdRule, counterpartyClientIdRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleConnectionOpenTryIterator{contract: _RelayerModule.contract, event: "ConnectionOpenTry", logs: logs, sub: sub}, nil
}

// WatchConnectionOpenTry is a free log subscription operation binding the contract event 0xa616a9aa2c65e935abbd15b07a9b5ff6c9c48b06b460a39b0b8cfda2a985869f.
//
// Solidity: event ConnectionOpenTry(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) WatchConnectionOpenTry(opts *bind.WatchOpts, sink chan<- *RelayerModuleConnectionOpenTry, connectionId []string, clientId []string, counterpartyClientId []string) (event.Subscription, error) {

	var connectionIdRule []interface{}
	for _, connectionIdItem := range connectionId {
		connectionIdRule = append(connectionIdRule, connectionIdItem)
	}
	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var counterpartyClientIdRule []interface{}
	for _, counterpartyClientIdItem := range counterpartyClientId {
		counterpartyClientIdRule = append(counterpartyClientIdRule, counterpartyClientIdItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "ConnectionOpenTry", connectionIdRule, clientIdRule, counterpartyClientIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleConnectionOpenTry)
				if err := _RelayerModule.contract.UnpackLog(event, "ConnectionOpenTry", log); err != nil {
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

// ParseConnectionOpenTry is a log parse operation binding the contract event 0xa616a9aa2c65e935abbd15b07a9b5ff6c9c48b06b460a39b0b8cfda2a985869f.
//
// Solidity: event ConnectionOpenTry(string indexed connectionId, string indexed clientId, string indexed counterpartyClientId, string counterpartyConnectionId)
func (_RelayerModule *RelayerModuleFilterer) ParseConnectionOpenTry(log types.Log) (*RelayerModuleConnectionOpenTry, error) {
	event := new(RelayerModuleConnectionOpenTry)
	if err := _RelayerModule.contract.UnpackLog(event, "ConnectionOpenTry", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleCreateClientIterator is returned from FilterCreateClient and is used to iterate over the raw logs and unpacked data for CreateClient events raised by the RelayerModule contract.
type RelayerModuleCreateClientIterator struct {
	Event *RelayerModuleCreateClient // Event containing the contract specifics and raw log

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
func (it *RelayerModuleCreateClientIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleCreateClient)
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
		it.Event = new(RelayerModuleCreateClient)
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
func (it *RelayerModuleCreateClientIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleCreateClientIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleCreateClient represents a CreateClient event raised by the RelayerModule contract.
type RelayerModuleCreateClient struct {
	ClientId   common.Hash
	ClientType common.Hash
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterCreateClient is a free log retrieval operation binding the contract event 0x99c72989fdc811e2ff3a5265b08f636a887faf5cbc61a014d2ac521a77421e8a.
//
// Solidity: event CreateClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) FilterCreateClient(opts *bind.FilterOpts, clientId []string, clientType []string) (*RelayerModuleCreateClientIterator, error) {

	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var clientTypeRule []interface{}
	for _, clientTypeItem := range clientType {
		clientTypeRule = append(clientTypeRule, clientTypeItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "CreateClient", clientIdRule, clientTypeRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleCreateClientIterator{contract: _RelayerModule.contract, event: "CreateClient", logs: logs, sub: sub}, nil
}

// WatchCreateClient is a free log subscription operation binding the contract event 0x99c72989fdc811e2ff3a5265b08f636a887faf5cbc61a014d2ac521a77421e8a.
//
// Solidity: event CreateClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) WatchCreateClient(opts *bind.WatchOpts, sink chan<- *RelayerModuleCreateClient, clientId []string, clientType []string) (event.Subscription, error) {

	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var clientTypeRule []interface{}
	for _, clientTypeItem := range clientType {
		clientTypeRule = append(clientTypeRule, clientTypeItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "CreateClient", clientIdRule, clientTypeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleCreateClient)
				if err := _RelayerModule.contract.UnpackLog(event, "CreateClient", log); err != nil {
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

// ParseCreateClient is a log parse operation binding the contract event 0x99c72989fdc811e2ff3a5265b08f636a887faf5cbc61a014d2ac521a77421e8a.
//
// Solidity: event CreateClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) ParseCreateClient(log types.Log) (*RelayerModuleCreateClient, error) {
	event := new(RelayerModuleCreateClient)
	if err := _RelayerModule.contract.UnpackLog(event, "CreateClient", log); err != nil {
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
	Denom common.Hash
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterDenominationTrace is a free log retrieval operation binding the contract event 0x483180a024351f3ea4c4782eaadb34add715974648a3d47bbff4a7b76da20859.
//
// Solidity: event DenominationTrace(string indexed denom)
func (_RelayerModule *RelayerModuleFilterer) FilterDenominationTrace(opts *bind.FilterOpts, denom []string) (*RelayerModuleDenominationTraceIterator, error) {

	var denomRule []interface{}
	for _, denomItem := range denom {
		denomRule = append(denomRule, denomItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "DenominationTrace", denomRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleDenominationTraceIterator{contract: _RelayerModule.contract, event: "DenominationTrace", logs: logs, sub: sub}, nil
}

// WatchDenominationTrace is a free log subscription operation binding the contract event 0x483180a024351f3ea4c4782eaadb34add715974648a3d47bbff4a7b76da20859.
//
// Solidity: event DenominationTrace(string indexed denom)
func (_RelayerModule *RelayerModuleFilterer) WatchDenominationTrace(opts *bind.WatchOpts, sink chan<- *RelayerModuleDenominationTrace, denom []string) (event.Subscription, error) {

	var denomRule []interface{}
	for _, denomItem := range denom {
		denomRule = append(denomRule, denomItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "DenominationTrace", denomRule)
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
// Solidity: event DenominationTrace(string indexed denom)
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
	Fee      common.Hash
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterDistributeFee is a free log retrieval operation binding the contract event 0x67e2bceb7881996b4bbddf9ab5d5c9bceb0ace3a06538b5e40be96094c4c9a72.
//
// Solidity: event DistributeFee(address indexed receiver, string indexed fee)
func (_RelayerModule *RelayerModuleFilterer) FilterDistributeFee(opts *bind.FilterOpts, receiver []common.Address, fee []string) (*RelayerModuleDistributeFeeIterator, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var feeRule []interface{}
	for _, feeItem := range fee {
		feeRule = append(feeRule, feeItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "DistributeFee", receiverRule, feeRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleDistributeFeeIterator{contract: _RelayerModule.contract, event: "DistributeFee", logs: logs, sub: sub}, nil
}

// WatchDistributeFee is a free log subscription operation binding the contract event 0x67e2bceb7881996b4bbddf9ab5d5c9bceb0ace3a06538b5e40be96094c4c9a72.
//
// Solidity: event DistributeFee(address indexed receiver, string indexed fee)
func (_RelayerModule *RelayerModuleFilterer) WatchDistributeFee(opts *bind.WatchOpts, sink chan<- *RelayerModuleDistributeFee, receiver []common.Address, fee []string) (event.Subscription, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var feeRule []interface{}
	for _, feeItem := range fee {
		feeRule = append(feeRule, feeItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "DistributeFee", receiverRule, feeRule)
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
// Solidity: event DistributeFee(address indexed receiver, string indexed fee)
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
	Denom    common.Hash
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterFungibleTokenPacket is a free log retrieval operation binding the contract event 0xe0fdee6007dd2fb6acfd338163a4260f0abf107fc184f28b75c5b2c1be55f573.
//
// Solidity: event FungibleTokenPacket(address indexed receiver, address indexed sender, string indexed denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) FilterFungibleTokenPacket(opts *bind.FilterOpts, receiver []common.Address, sender []common.Address, denom []string) (*RelayerModuleFungibleTokenPacketIterator, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var denomRule []interface{}
	for _, denomItem := range denom {
		denomRule = append(denomRule, denomItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "FungibleTokenPacket", receiverRule, senderRule, denomRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleFungibleTokenPacketIterator{contract: _RelayerModule.contract, event: "FungibleTokenPacket", logs: logs, sub: sub}, nil
}

// WatchFungibleTokenPacket is a free log subscription operation binding the contract event 0xe0fdee6007dd2fb6acfd338163a4260f0abf107fc184f28b75c5b2c1be55f573.
//
// Solidity: event FungibleTokenPacket(address indexed receiver, address indexed sender, string indexed denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) WatchFungibleTokenPacket(opts *bind.WatchOpts, sink chan<- *RelayerModuleFungibleTokenPacket, receiver []common.Address, sender []common.Address, denom []string) (event.Subscription, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var denomRule []interface{}
	for _, denomItem := range denom {
		denomRule = append(denomRule, denomItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "FungibleTokenPacket", receiverRule, senderRule, denomRule)
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
// Solidity: event FungibleTokenPacket(address indexed receiver, address indexed sender, string indexed denom, uint256 amount)
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
	Denom    common.Hash
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterIbcTransfer is a free log retrieval operation binding the contract event 0x3015d0e0dd0cda983ea101408cb97c173ec3a038ee6f439cc3c7532c52057c0c.
//
// Solidity: event IbcTransfer(address indexed sender, address indexed receiver, string indexed denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) FilterIbcTransfer(opts *bind.FilterOpts, sender []common.Address, receiver []common.Address, denom []string) (*RelayerModuleIbcTransferIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var denomRule []interface{}
	for _, denomItem := range denom {
		denomRule = append(denomRule, denomItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "IbcTransfer", senderRule, receiverRule, denomRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleIbcTransferIterator{contract: _RelayerModule.contract, event: "IbcTransfer", logs: logs, sub: sub}, nil
}

// WatchIbcTransfer is a free log subscription operation binding the contract event 0x3015d0e0dd0cda983ea101408cb97c173ec3a038ee6f439cc3c7532c52057c0c.
//
// Solidity: event IbcTransfer(address indexed sender, address indexed receiver, string indexed denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) WatchIbcTransfer(opts *bind.WatchOpts, sink chan<- *RelayerModuleIbcTransfer, sender []common.Address, receiver []common.Address, denom []string) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var denomRule []interface{}
	for _, denomItem := range denom {
		denomRule = append(denomRule, denomItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "IbcTransfer", senderRule, receiverRule, denomRule)
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
// Solidity: event IbcTransfer(address indexed sender, address indexed receiver, string indexed denom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) ParseIbcTransfer(log types.Log) (*RelayerModuleIbcTransfer, error) {
	event := new(RelayerModuleIbcTransfer)
	if err := _RelayerModule.contract.UnpackLog(event, "IbcTransfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleMessageIterator is returned from FilterMessage and is used to iterate over the raw logs and unpacked data for Message events raised by the RelayerModule contract.
type RelayerModuleMessageIterator struct {
	Event *RelayerModuleMessage // Event containing the contract specifics and raw log

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
func (it *RelayerModuleMessageIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleMessage)
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
		it.Event = new(RelayerModuleMessage)
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
func (it *RelayerModuleMessageIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleMessageIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleMessage represents a Message event raised by the RelayerModule contract.
type RelayerModuleMessage struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterMessage is a free log retrieval operation binding the contract event 0xa1fb1de9edd1fb65c0511622fb0cd565140cf89f7579780ffa06863d994d4adb.
//
// Solidity: event Message()
func (_RelayerModule *RelayerModuleFilterer) FilterMessage(opts *bind.FilterOpts) (*RelayerModuleMessageIterator, error) {

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "Message")
	if err != nil {
		return nil, err
	}
	return &RelayerModuleMessageIterator{contract: _RelayerModule.contract, event: "Message", logs: logs, sub: sub}, nil
}

// WatchMessage is a free log subscription operation binding the contract event 0xa1fb1de9edd1fb65c0511622fb0cd565140cf89f7579780ffa06863d994d4adb.
//
// Solidity: event Message()
func (_RelayerModule *RelayerModuleFilterer) WatchMessage(opts *bind.WatchOpts, sink chan<- *RelayerModuleMessage) (event.Subscription, error) {

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "Message")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleMessage)
				if err := _RelayerModule.contract.UnpackLog(event, "Message", log); err != nil {
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

// ParseMessage is a log parse operation binding the contract event 0xa1fb1de9edd1fb65c0511622fb0cd565140cf89f7579780ffa06863d994d4adb.
//
// Solidity: event Message()
func (_RelayerModule *RelayerModuleFilterer) ParseMessage(log types.Log) (*RelayerModuleMessage, error) {
	event := new(RelayerModuleMessage)
	if err := _RelayerModule.contract.UnpackLog(event, "Message", log); err != nil {
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
	PacketData IRelayerModulePacketData
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterRecvPacket is a free log retrieval operation binding the contract event 0x9b65d2e7fb26fd6c5ea41ebf07b4360b58fb218c4e50aa439df57a4c082f9190.
//
// Solidity: event RecvPacket((address,string,(uint256,string)[]) packetData)
func (_RelayerModule *RelayerModuleFilterer) FilterRecvPacket(opts *bind.FilterOpts) (*RelayerModuleRecvPacketIterator, error) {

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "RecvPacket")
	if err != nil {
		return nil, err
	}
	return &RelayerModuleRecvPacketIterator{contract: _RelayerModule.contract, event: "RecvPacket", logs: logs, sub: sub}, nil
}

// WatchRecvPacket is a free log subscription operation binding the contract event 0x9b65d2e7fb26fd6c5ea41ebf07b4360b58fb218c4e50aa439df57a4c082f9190.
//
// Solidity: event RecvPacket((address,string,(uint256,string)[]) packetData)
func (_RelayerModule *RelayerModuleFilterer) WatchRecvPacket(opts *bind.WatchOpts, sink chan<- *RelayerModuleRecvPacket) (event.Subscription, error) {

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "RecvPacket")
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

// ParseRecvPacket is a log parse operation binding the contract event 0x9b65d2e7fb26fd6c5ea41ebf07b4360b58fb218c4e50aa439df57a4c082f9190.
//
// Solidity: event RecvPacket((address,string,(uint256,string)[]) packetData)
func (_RelayerModule *RelayerModuleFilterer) ParseRecvPacket(log types.Log) (*RelayerModuleRecvPacket, error) {
	event := new(RelayerModuleRecvPacket)
	if err := _RelayerModule.contract.UnpackLog(event, "RecvPacket", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleSubmitMisbehaviourIterator is returned from FilterSubmitMisbehaviour and is used to iterate over the raw logs and unpacked data for SubmitMisbehaviour events raised by the RelayerModule contract.
type RelayerModuleSubmitMisbehaviourIterator struct {
	Event *RelayerModuleSubmitMisbehaviour // Event containing the contract specifics and raw log

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
func (it *RelayerModuleSubmitMisbehaviourIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleSubmitMisbehaviour)
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
		it.Event = new(RelayerModuleSubmitMisbehaviour)
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
func (it *RelayerModuleSubmitMisbehaviourIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleSubmitMisbehaviourIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleSubmitMisbehaviour represents a SubmitMisbehaviour event raised by the RelayerModule contract.
type RelayerModuleSubmitMisbehaviour struct {
	SubjectId  common.Hash
	ClientType common.Hash
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSubmitMisbehaviour is a free log retrieval operation binding the contract event 0x5e3d4bfbdf00af6a11dfe40554bb939eca3f49763f519b0c2fb78b7277911029.
//
// Solidity: event SubmitMisbehaviour(string indexed subjectId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) FilterSubmitMisbehaviour(opts *bind.FilterOpts, subjectId []string, clientType []string) (*RelayerModuleSubmitMisbehaviourIterator, error) {

	var subjectIdRule []interface{}
	for _, subjectIdItem := range subjectId {
		subjectIdRule = append(subjectIdRule, subjectIdItem)
	}
	var clientTypeRule []interface{}
	for _, clientTypeItem := range clientType {
		clientTypeRule = append(clientTypeRule, clientTypeItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "SubmitMisbehaviour", subjectIdRule, clientTypeRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleSubmitMisbehaviourIterator{contract: _RelayerModule.contract, event: "SubmitMisbehaviour", logs: logs, sub: sub}, nil
}

// WatchSubmitMisbehaviour is a free log subscription operation binding the contract event 0x5e3d4bfbdf00af6a11dfe40554bb939eca3f49763f519b0c2fb78b7277911029.
//
// Solidity: event SubmitMisbehaviour(string indexed subjectId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) WatchSubmitMisbehaviour(opts *bind.WatchOpts, sink chan<- *RelayerModuleSubmitMisbehaviour, subjectId []string, clientType []string) (event.Subscription, error) {

	var subjectIdRule []interface{}
	for _, subjectIdItem := range subjectId {
		subjectIdRule = append(subjectIdRule, subjectIdItem)
	}
	var clientTypeRule []interface{}
	for _, clientTypeItem := range clientType {
		clientTypeRule = append(clientTypeRule, clientTypeItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "SubmitMisbehaviour", subjectIdRule, clientTypeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleSubmitMisbehaviour)
				if err := _RelayerModule.contract.UnpackLog(event, "SubmitMisbehaviour", log); err != nil {
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

// ParseSubmitMisbehaviour is a log parse operation binding the contract event 0x5e3d4bfbdf00af6a11dfe40554bb939eca3f49763f519b0c2fb78b7277911029.
//
// Solidity: event SubmitMisbehaviour(string indexed subjectId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) ParseSubmitMisbehaviour(log types.Log) (*RelayerModuleSubmitMisbehaviour, error) {
	event := new(RelayerModuleSubmitMisbehaviour)
	if err := _RelayerModule.contract.UnpackLog(event, "SubmitMisbehaviour", log); err != nil {
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
	RefundDenom    common.Hash
	Amount         *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterTimeout is a free log retrieval operation binding the contract event 0x1d76f8448fb37cb7d524e0c59091293b9c39eea4c53674d9f53f323fee1b971f.
//
// Solidity: event Timeout(address indexed refundReceiver, string indexed refundDenom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) FilterTimeout(opts *bind.FilterOpts, refundReceiver []common.Address, refundDenom []string) (*RelayerModuleTimeoutIterator, error) {

	var refundReceiverRule []interface{}
	for _, refundReceiverItem := range refundReceiver {
		refundReceiverRule = append(refundReceiverRule, refundReceiverItem)
	}
	var refundDenomRule []interface{}
	for _, refundDenomItem := range refundDenom {
		refundDenomRule = append(refundDenomRule, refundDenomItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "Timeout", refundReceiverRule, refundDenomRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleTimeoutIterator{contract: _RelayerModule.contract, event: "Timeout", logs: logs, sub: sub}, nil
}

// WatchTimeout is a free log subscription operation binding the contract event 0x1d76f8448fb37cb7d524e0c59091293b9c39eea4c53674d9f53f323fee1b971f.
//
// Solidity: event Timeout(address indexed refundReceiver, string indexed refundDenom, uint256 amount)
func (_RelayerModule *RelayerModuleFilterer) WatchTimeout(opts *bind.WatchOpts, sink chan<- *RelayerModuleTimeout, refundReceiver []common.Address, refundDenom []string) (event.Subscription, error) {

	var refundReceiverRule []interface{}
	for _, refundReceiverItem := range refundReceiver {
		refundReceiverRule = append(refundReceiverRule, refundReceiverItem)
	}
	var refundDenomRule []interface{}
	for _, refundDenomItem := range refundDenom {
		refundDenomRule = append(refundDenomRule, refundDenomItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "Timeout", refundReceiverRule, refundDenomRule)
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

// ParseTimeout is a log parse operation binding the contract event 0x1d76f8448fb37cb7d524e0c59091293b9c39eea4c53674d9f53f323fee1b971f.
//
// Solidity: event Timeout(address indexed refundReceiver, string indexed refundDenom, uint256 amount)
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
	PacketSrcPort         common.Hash
	PacketSrcChannel      common.Hash
	PacketDstPort         common.Hash
	PacketDstChannel      string
	PacketChannelOrdering string
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterTimeoutPacket is a free log retrieval operation binding the contract event 0x15785d3f86870204161ea96f3a26ee3848d7732767e6aa943ab88f45efc1cca0.
//
// Solidity: event TimeoutPacket(string indexed packetSrcPort, string indexed packetSrcChannel, string indexed packetDstPort, string packetDstChannel, string packetChannelOrdering)
func (_RelayerModule *RelayerModuleFilterer) FilterTimeoutPacket(opts *bind.FilterOpts, packetSrcPort []string, packetSrcChannel []string, packetDstPort []string) (*RelayerModuleTimeoutPacketIterator, error) {

	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}
	var packetDstPortRule []interface{}
	for _, packetDstPortItem := range packetDstPort {
		packetDstPortRule = append(packetDstPortRule, packetDstPortItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "TimeoutPacket", packetSrcPortRule, packetSrcChannelRule, packetDstPortRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleTimeoutPacketIterator{contract: _RelayerModule.contract, event: "TimeoutPacket", logs: logs, sub: sub}, nil
}

// WatchTimeoutPacket is a free log subscription operation binding the contract event 0x15785d3f86870204161ea96f3a26ee3848d7732767e6aa943ab88f45efc1cca0.
//
// Solidity: event TimeoutPacket(string indexed packetSrcPort, string indexed packetSrcChannel, string indexed packetDstPort, string packetDstChannel, string packetChannelOrdering)
func (_RelayerModule *RelayerModuleFilterer) WatchTimeoutPacket(opts *bind.WatchOpts, sink chan<- *RelayerModuleTimeoutPacket, packetSrcPort []string, packetSrcChannel []string, packetDstPort []string) (event.Subscription, error) {

	var packetSrcPortRule []interface{}
	for _, packetSrcPortItem := range packetSrcPort {
		packetSrcPortRule = append(packetSrcPortRule, packetSrcPortItem)
	}
	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}
	var packetDstPortRule []interface{}
	for _, packetDstPortItem := range packetDstPort {
		packetDstPortRule = append(packetDstPortRule, packetDstPortItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "TimeoutPacket", packetSrcPortRule, packetSrcChannelRule, packetDstPortRule)
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

// ParseTimeoutPacket is a log parse operation binding the contract event 0x15785d3f86870204161ea96f3a26ee3848d7732767e6aa943ab88f45efc1cca0.
//
// Solidity: event TimeoutPacket(string indexed packetSrcPort, string indexed packetSrcChannel, string indexed packetDstPort, string packetDstChannel, string packetChannelOrdering)
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

// RelayerModuleUpdateClientIterator is returned from FilterUpdateClient and is used to iterate over the raw logs and unpacked data for UpdateClient events raised by the RelayerModule contract.
type RelayerModuleUpdateClientIterator struct {
	Event *RelayerModuleUpdateClient // Event containing the contract specifics and raw log

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
func (it *RelayerModuleUpdateClientIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleUpdateClient)
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
		it.Event = new(RelayerModuleUpdateClient)
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
func (it *RelayerModuleUpdateClientIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleUpdateClientIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleUpdateClient represents a UpdateClient event raised by the RelayerModule contract.
type RelayerModuleUpdateClient struct {
	ClientId   common.Hash
	ClientType common.Hash
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterUpdateClient is a free log retrieval operation binding the contract event 0x96cbed941dd0e14249990cc56370fd97652ced76f675a2d8ce36b9545a000583.
//
// Solidity: event UpdateClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) FilterUpdateClient(opts *bind.FilterOpts, clientId []string, clientType []string) (*RelayerModuleUpdateClientIterator, error) {

	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var clientTypeRule []interface{}
	for _, clientTypeItem := range clientType {
		clientTypeRule = append(clientTypeRule, clientTypeItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "UpdateClient", clientIdRule, clientTypeRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleUpdateClientIterator{contract: _RelayerModule.contract, event: "UpdateClient", logs: logs, sub: sub}, nil
}

// WatchUpdateClient is a free log subscription operation binding the contract event 0x96cbed941dd0e14249990cc56370fd97652ced76f675a2d8ce36b9545a000583.
//
// Solidity: event UpdateClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) WatchUpdateClient(opts *bind.WatchOpts, sink chan<- *RelayerModuleUpdateClient, clientId []string, clientType []string) (event.Subscription, error) {

	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var clientTypeRule []interface{}
	for _, clientTypeItem := range clientType {
		clientTypeRule = append(clientTypeRule, clientTypeItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "UpdateClient", clientIdRule, clientTypeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleUpdateClient)
				if err := _RelayerModule.contract.UnpackLog(event, "UpdateClient", log); err != nil {
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

// ParseUpdateClient is a log parse operation binding the contract event 0x96cbed941dd0e14249990cc56370fd97652ced76f675a2d8ce36b9545a000583.
//
// Solidity: event UpdateClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) ParseUpdateClient(log types.Log) (*RelayerModuleUpdateClient, error) {
	event := new(RelayerModuleUpdateClient)
	if err := _RelayerModule.contract.UnpackLog(event, "UpdateClient", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RelayerModuleUpgradeClientIterator is returned from FilterUpgradeClient and is used to iterate over the raw logs and unpacked data for UpgradeClient events raised by the RelayerModule contract.
type RelayerModuleUpgradeClientIterator struct {
	Event *RelayerModuleUpgradeClient // Event containing the contract specifics and raw log

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
func (it *RelayerModuleUpgradeClientIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayerModuleUpgradeClient)
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
		it.Event = new(RelayerModuleUpgradeClient)
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
func (it *RelayerModuleUpgradeClientIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayerModuleUpgradeClientIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayerModuleUpgradeClient represents a UpgradeClient event raised by the RelayerModule contract.
type RelayerModuleUpgradeClient struct {
	ClientId   common.Hash
	ClientType common.Hash
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterUpgradeClient is a free log retrieval operation binding the contract event 0xea94737b2a063148360d4e7658f21a26653e4cd80dd2142c2ea96422be27e2e4.
//
// Solidity: event UpgradeClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) FilterUpgradeClient(opts *bind.FilterOpts, clientId []string, clientType []string) (*RelayerModuleUpgradeClientIterator, error) {

	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var clientTypeRule []interface{}
	for _, clientTypeItem := range clientType {
		clientTypeRule = append(clientTypeRule, clientTypeItem)
	}

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "UpgradeClient", clientIdRule, clientTypeRule)
	if err != nil {
		return nil, err
	}
	return &RelayerModuleUpgradeClientIterator{contract: _RelayerModule.contract, event: "UpgradeClient", logs: logs, sub: sub}, nil
}

// WatchUpgradeClient is a free log subscription operation binding the contract event 0xea94737b2a063148360d4e7658f21a26653e4cd80dd2142c2ea96422be27e2e4.
//
// Solidity: event UpgradeClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) WatchUpgradeClient(opts *bind.WatchOpts, sink chan<- *RelayerModuleUpgradeClient, clientId []string, clientType []string) (event.Subscription, error) {

	var clientIdRule []interface{}
	for _, clientIdItem := range clientId {
		clientIdRule = append(clientIdRule, clientIdItem)
	}
	var clientTypeRule []interface{}
	for _, clientTypeItem := range clientType {
		clientTypeRule = append(clientTypeRule, clientTypeItem)
	}

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "UpgradeClient", clientIdRule, clientTypeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayerModuleUpgradeClient)
				if err := _RelayerModule.contract.UnpackLog(event, "UpgradeClient", log); err != nil {
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

// ParseUpgradeClient is a log parse operation binding the contract event 0xea94737b2a063148360d4e7658f21a26653e4cd80dd2142c2ea96422be27e2e4.
//
// Solidity: event UpgradeClient(string indexed clientId, string indexed clientType)
func (_RelayerModule *RelayerModuleFilterer) ParseUpgradeClient(log types.Log) (*RelayerModuleUpgradeClient, error) {
	event := new(RelayerModuleUpgradeClient)
	if err := _RelayerModule.contract.UnpackLog(event, "UpgradeClient", log); err != nil {
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
	PacketConnection string
	PacketData       IRelayerModulePacketData
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterWriteAcknowledgement is a free log retrieval operation binding the contract event 0x4adabcca51310729667d527f01d2da2f2dd3f0a132db55702fe874d878eaf856.
//
// Solidity: event WriteAcknowledgement(string packetConnection, (address,string,(uint256,string)[]) packetData)
func (_RelayerModule *RelayerModuleFilterer) FilterWriteAcknowledgement(opts *bind.FilterOpts) (*RelayerModuleWriteAcknowledgementIterator, error) {

	logs, sub, err := _RelayerModule.contract.FilterLogs(opts, "WriteAcknowledgement")
	if err != nil {
		return nil, err
	}
	return &RelayerModuleWriteAcknowledgementIterator{contract: _RelayerModule.contract, event: "WriteAcknowledgement", logs: logs, sub: sub}, nil
}

// WatchWriteAcknowledgement is a free log subscription operation binding the contract event 0x4adabcca51310729667d527f01d2da2f2dd3f0a132db55702fe874d878eaf856.
//
// Solidity: event WriteAcknowledgement(string packetConnection, (address,string,(uint256,string)[]) packetData)
func (_RelayerModule *RelayerModuleFilterer) WatchWriteAcknowledgement(opts *bind.WatchOpts, sink chan<- *RelayerModuleWriteAcknowledgement) (event.Subscription, error) {

	logs, sub, err := _RelayerModule.contract.WatchLogs(opts, "WriteAcknowledgement")
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

// ParseWriteAcknowledgement is a log parse operation binding the contract event 0x4adabcca51310729667d527f01d2da2f2dd3f0a132db55702fe874d878eaf856.
//
// Solidity: event WriteAcknowledgement(string packetConnection, (address,string,(uint256,string)[]) packetData)
func (_RelayerModule *RelayerModuleFilterer) ParseWriteAcknowledgement(log types.Log) (*RelayerModuleWriteAcknowledgement, error) {
	event := new(RelayerModuleWriteAcknowledgement)
	if err := _RelayerModule.contract.UnpackLog(event, "WriteAcknowledgement", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
