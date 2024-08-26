// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package ica

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

// ICAModuleMetaData contains all meta data concerning the ICAModule contract.
var ICAModuleMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"string\",\"name\":\"packetSrcChannel\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"seq\",\"type\":\"uint64\"}],\"name\":\"SubmitMsgsResult\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"connectionID\",\"type\":\"string\"},{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"queryAccount\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"connectionID\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"version\",\"type\":\"string\"},{\"internalType\":\"int32\",\"name\":\"ordering\",\"type\":\"int32\"}],\"name\":\"registerAccount\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"connectionID\",\"type\":\"string\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"timeout\",\"type\":\"uint256\"}],\"name\":\"submitMsgs\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// ICAModuleABI is the input ABI used to generate the binding from.
// Deprecated: Use ICAModuleMetaData.ABI instead.
var ICAModuleABI = ICAModuleMetaData.ABI

// ICAModule is an auto generated Go binding around an Ethereum contract.
type ICAModule struct {
	ICAModuleCaller     // Read-only binding to the contract
	ICAModuleTransactor // Write-only binding to the contract
	ICAModuleFilterer   // Log filterer for contract events
}

// ICAModuleCaller is an auto generated read-only Go binding around an Ethereum contract.
type ICAModuleCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ICAModuleTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ICAModuleTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ICAModuleFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ICAModuleFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ICAModuleSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ICAModuleSession struct {
	Contract     *ICAModule        // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ICAModuleCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ICAModuleCallerSession struct {
	Contract *ICAModuleCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts    // Call options to use throughout this session
}

// ICAModuleTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ICAModuleTransactorSession struct {
	Contract     *ICAModuleTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts    // Transaction auth options to use throughout this session
}

// ICAModuleRaw is an auto generated low-level Go binding around an Ethereum contract.
type ICAModuleRaw struct {
	Contract *ICAModule // Generic contract binding to access the raw methods on
}

// ICAModuleCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ICAModuleCallerRaw struct {
	Contract *ICAModuleCaller // Generic read-only contract binding to access the raw methods on
}

// ICAModuleTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ICAModuleTransactorRaw struct {
	Contract *ICAModuleTransactor // Generic write-only contract binding to access the raw methods on
}

// NewICAModule creates a new instance of ICAModule, bound to a specific deployed contract.
func NewICAModule(address common.Address, backend bind.ContractBackend) (*ICAModule, error) {
	contract, err := bindICAModule(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ICAModule{ICAModuleCaller: ICAModuleCaller{contract: contract}, ICAModuleTransactor: ICAModuleTransactor{contract: contract}, ICAModuleFilterer: ICAModuleFilterer{contract: contract}}, nil
}

// NewICAModuleCaller creates a new read-only instance of ICAModule, bound to a specific deployed contract.
func NewICAModuleCaller(address common.Address, caller bind.ContractCaller) (*ICAModuleCaller, error) {
	contract, err := bindICAModule(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ICAModuleCaller{contract: contract}, nil
}

// NewICAModuleTransactor creates a new write-only instance of ICAModule, bound to a specific deployed contract.
func NewICAModuleTransactor(address common.Address, transactor bind.ContractTransactor) (*ICAModuleTransactor, error) {
	contract, err := bindICAModule(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ICAModuleTransactor{contract: contract}, nil
}

// NewICAModuleFilterer creates a new log filterer instance of ICAModule, bound to a specific deployed contract.
func NewICAModuleFilterer(address common.Address, filterer bind.ContractFilterer) (*ICAModuleFilterer, error) {
	contract, err := bindICAModule(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ICAModuleFilterer{contract: contract}, nil
}

// bindICAModule binds a generic wrapper to an already deployed contract.
func bindICAModule(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ICAModuleMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ICAModule *ICAModuleRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ICAModule.Contract.ICAModuleCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ICAModule *ICAModuleRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ICAModule.Contract.ICAModuleTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ICAModule *ICAModuleRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ICAModule.Contract.ICAModuleTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ICAModule *ICAModuleCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ICAModule.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ICAModule *ICAModuleTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ICAModule.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ICAModule *ICAModuleTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ICAModule.Contract.contract.Transact(opts, method, params...)
}

// QueryAccount is a free data retrieval call binding the contract method 0x15bf8a47.
//
// Solidity: function queryAccount(string connectionID, address addr) view returns(string)
func (_ICAModule *ICAModuleCaller) QueryAccount(opts *bind.CallOpts, connectionID string, addr common.Address) (string, error) {
	var out []interface{}
	err := _ICAModule.contract.Call(opts, &out, "queryAccount", connectionID, addr)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// QueryAccount is a free data retrieval call binding the contract method 0x15bf8a47.
//
// Solidity: function queryAccount(string connectionID, address addr) view returns(string)
func (_ICAModule *ICAModuleSession) QueryAccount(connectionID string, addr common.Address) (string, error) {
	return _ICAModule.Contract.QueryAccount(&_ICAModule.CallOpts, connectionID, addr)
}

// QueryAccount is a free data retrieval call binding the contract method 0x15bf8a47.
//
// Solidity: function queryAccount(string connectionID, address addr) view returns(string)
func (_ICAModule *ICAModuleCallerSession) QueryAccount(connectionID string, addr common.Address) (string, error) {
	return _ICAModule.Contract.QueryAccount(&_ICAModule.CallOpts, connectionID, addr)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x9a20d545.
//
// Solidity: function registerAccount(string connectionID, string version, int32 ordering) payable returns(bool)
func (_ICAModule *ICAModuleTransactor) RegisterAccount(opts *bind.TransactOpts, connectionID string, version string, ordering int32) (*types.Transaction, error) {
	return _ICAModule.contract.Transact(opts, "registerAccount", connectionID, version, ordering)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x9a20d545.
//
// Solidity: function registerAccount(string connectionID, string version, int32 ordering) payable returns(bool)
func (_ICAModule *ICAModuleSession) RegisterAccount(connectionID string, version string, ordering int32) (*types.Transaction, error) {
	return _ICAModule.Contract.RegisterAccount(&_ICAModule.TransactOpts, connectionID, version, ordering)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x9a20d545.
//
// Solidity: function registerAccount(string connectionID, string version, int32 ordering) payable returns(bool)
func (_ICAModule *ICAModuleTransactorSession) RegisterAccount(connectionID string, version string, ordering int32) (*types.Transaction, error) {
	return _ICAModule.Contract.RegisterAccount(&_ICAModule.TransactOpts, connectionID, version, ordering)
}

// SubmitMsgs is a paid mutator transaction binding the contract method 0x697bfa34.
//
// Solidity: function submitMsgs(string connectionID, bytes data, uint256 timeout) payable returns(uint64)
func (_ICAModule *ICAModuleTransactor) SubmitMsgs(opts *bind.TransactOpts, connectionID string, data []byte, timeout *big.Int) (*types.Transaction, error) {
	return _ICAModule.contract.Transact(opts, "submitMsgs", connectionID, data, timeout)
}

// SubmitMsgs is a paid mutator transaction binding the contract method 0x697bfa34.
//
// Solidity: function submitMsgs(string connectionID, bytes data, uint256 timeout) payable returns(uint64)
func (_ICAModule *ICAModuleSession) SubmitMsgs(connectionID string, data []byte, timeout *big.Int) (*types.Transaction, error) {
	return _ICAModule.Contract.SubmitMsgs(&_ICAModule.TransactOpts, connectionID, data, timeout)
}

// SubmitMsgs is a paid mutator transaction binding the contract method 0x697bfa34.
//
// Solidity: function submitMsgs(string connectionID, bytes data, uint256 timeout) payable returns(uint64)
func (_ICAModule *ICAModuleTransactorSession) SubmitMsgs(connectionID string, data []byte, timeout *big.Int) (*types.Transaction, error) {
	return _ICAModule.Contract.SubmitMsgs(&_ICAModule.TransactOpts, connectionID, data, timeout)
}

// ICAModuleSubmitMsgsResultIterator is returned from FilterSubmitMsgsResult and is used to iterate over the raw logs and unpacked data for SubmitMsgsResult events raised by the ICAModule contract.
type ICAModuleSubmitMsgsResultIterator struct {
	Event *ICAModuleSubmitMsgsResult // Event containing the contract specifics and raw log

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
func (it *ICAModuleSubmitMsgsResultIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ICAModuleSubmitMsgsResult)
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
		it.Event = new(ICAModuleSubmitMsgsResult)
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
func (it *ICAModuleSubmitMsgsResultIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ICAModuleSubmitMsgsResultIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ICAModuleSubmitMsgsResult represents a SubmitMsgsResult event raised by the ICAModule contract.
type ICAModuleSubmitMsgsResult struct {
	PacketSrcChannel common.Hash
	Seq              uint64
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterSubmitMsgsResult is a free log retrieval operation binding the contract event 0x3e695f8f5525f556604458a25db1d059820bd331e56bc17b0693b9d35e243eca.
//
// Solidity: event SubmitMsgsResult(string indexed packetSrcChannel, uint64 seq)
func (_ICAModule *ICAModuleFilterer) FilterSubmitMsgsResult(opts *bind.FilterOpts, packetSrcChannel []string) (*ICAModuleSubmitMsgsResultIterator, error) {

	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _ICAModule.contract.FilterLogs(opts, "SubmitMsgsResult", packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return &ICAModuleSubmitMsgsResultIterator{contract: _ICAModule.contract, event: "SubmitMsgsResult", logs: logs, sub: sub}, nil
}

// WatchSubmitMsgsResult is a free log subscription operation binding the contract event 0x3e695f8f5525f556604458a25db1d059820bd331e56bc17b0693b9d35e243eca.
//
// Solidity: event SubmitMsgsResult(string indexed packetSrcChannel, uint64 seq)
func (_ICAModule *ICAModuleFilterer) WatchSubmitMsgsResult(opts *bind.WatchOpts, sink chan<- *ICAModuleSubmitMsgsResult, packetSrcChannel []string) (event.Subscription, error) {

	var packetSrcChannelRule []interface{}
	for _, packetSrcChannelItem := range packetSrcChannel {
		packetSrcChannelRule = append(packetSrcChannelRule, packetSrcChannelItem)
	}

	logs, sub, err := _ICAModule.contract.WatchLogs(opts, "SubmitMsgsResult", packetSrcChannelRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ICAModuleSubmitMsgsResult)
				if err := _ICAModule.contract.UnpackLog(event, "SubmitMsgsResult", log); err != nil {
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

// ParseSubmitMsgsResult is a log parse operation binding the contract event 0x3e695f8f5525f556604458a25db1d059820bd331e56bc17b0693b9d35e243eca.
//
// Solidity: event SubmitMsgsResult(string indexed packetSrcChannel, uint64 seq)
func (_ICAModule *ICAModuleFilterer) ParseSubmitMsgsResult(log types.Log) (*ICAModuleSubmitMsgsResult, error) {
	event := new(ICAModuleSubmitMsgsResult)
	if err := _ICAModule.contract.UnpackLog(event, "SubmitMsgsResult", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
