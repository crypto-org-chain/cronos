// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package llama

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

// ILLamaModuleMetaData contains all meta data concerning the ILLamaModule contract.
var ILLamaModuleMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"result\",\"type\":\"string\"}],\"name\":\"InferenceResult\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"prompt\",\"type\":\"string\"},{\"internalType\":\"int64\",\"name\":\"seed\",\"type\":\"int64\"},{\"internalType\":\"int32\",\"name\":\"steps\",\"type\":\"int32\"}],\"name\":\"inference\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"result\",\"type\":\"string\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// ILLamaModuleABI is the input ABI used to generate the binding from.
// Deprecated: Use ILLamaModuleMetaData.ABI instead.
var ILLamaModuleABI = ILLamaModuleMetaData.ABI

// ILLamaModule is an auto generated Go binding around an Ethereum contract.
type ILLamaModule struct {
	ILLamaModuleCaller     // Read-only binding to the contract
	ILLamaModuleTransactor // Write-only binding to the contract
	ILLamaModuleFilterer   // Log filterer for contract events
}

// ILLamaModuleCaller is an auto generated read-only Go binding around an Ethereum contract.
type ILLamaModuleCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ILLamaModuleTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ILLamaModuleTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ILLamaModuleFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ILLamaModuleFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ILLamaModuleSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ILLamaModuleSession struct {
	Contract     *ILLamaModule     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ILLamaModuleCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ILLamaModuleCallerSession struct {
	Contract *ILLamaModuleCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// ILLamaModuleTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ILLamaModuleTransactorSession struct {
	Contract     *ILLamaModuleTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// ILLamaModuleRaw is an auto generated low-level Go binding around an Ethereum contract.
type ILLamaModuleRaw struct {
	Contract *ILLamaModule // Generic contract binding to access the raw methods on
}

// ILLamaModuleCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ILLamaModuleCallerRaw struct {
	Contract *ILLamaModuleCaller // Generic read-only contract binding to access the raw methods on
}

// ILLamaModuleTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ILLamaModuleTransactorRaw struct {
	Contract *ILLamaModuleTransactor // Generic write-only contract binding to access the raw methods on
}

// NewILLamaModule creates a new instance of ILLamaModule, bound to a specific deployed contract.
func NewILLamaModule(address common.Address, backend bind.ContractBackend) (*ILLamaModule, error) {
	contract, err := bindILLamaModule(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ILLamaModule{ILLamaModuleCaller: ILLamaModuleCaller{contract: contract}, ILLamaModuleTransactor: ILLamaModuleTransactor{contract: contract}, ILLamaModuleFilterer: ILLamaModuleFilterer{contract: contract}}, nil
}

// NewILLamaModuleCaller creates a new read-only instance of ILLamaModule, bound to a specific deployed contract.
func NewILLamaModuleCaller(address common.Address, caller bind.ContractCaller) (*ILLamaModuleCaller, error) {
	contract, err := bindILLamaModule(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ILLamaModuleCaller{contract: contract}, nil
}

// NewILLamaModuleTransactor creates a new write-only instance of ILLamaModule, bound to a specific deployed contract.
func NewILLamaModuleTransactor(address common.Address, transactor bind.ContractTransactor) (*ILLamaModuleTransactor, error) {
	contract, err := bindILLamaModule(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ILLamaModuleTransactor{contract: contract}, nil
}

// NewILLamaModuleFilterer creates a new log filterer instance of ILLamaModule, bound to a specific deployed contract.
func NewILLamaModuleFilterer(address common.Address, filterer bind.ContractFilterer) (*ILLamaModuleFilterer, error) {
	contract, err := bindILLamaModule(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ILLamaModuleFilterer{contract: contract}, nil
}

// bindILLamaModule binds a generic wrapper to an already deployed contract.
func bindILLamaModule(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ILLamaModuleMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ILLamaModule *ILLamaModuleRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ILLamaModule.Contract.ILLamaModuleCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ILLamaModule *ILLamaModuleRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ILLamaModule.Contract.ILLamaModuleTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ILLamaModule *ILLamaModuleRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ILLamaModule.Contract.ILLamaModuleTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ILLamaModule *ILLamaModuleCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ILLamaModule.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ILLamaModule *ILLamaModuleTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ILLamaModule.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ILLamaModule *ILLamaModuleTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ILLamaModule.Contract.contract.Transact(opts, method, params...)
}

// Inference is a paid mutator transaction binding the contract method 0x3c8c648f.
//
// Solidity: function inference(string prompt, int64 seed, int32 steps) payable returns(string result)
func (_ILLamaModule *ILLamaModuleTransactor) Inference(opts *bind.TransactOpts, prompt string, seed int64, steps int32) (*types.Transaction, error) {
	return _ILLamaModule.contract.Transact(opts, "inference", prompt, seed, steps)
}

// Inference is a paid mutator transaction binding the contract method 0x3c8c648f.
//
// Solidity: function inference(string prompt, int64 seed, int32 steps) payable returns(string result)
func (_ILLamaModule *ILLamaModuleSession) Inference(prompt string, seed int64, steps int32) (*types.Transaction, error) {
	return _ILLamaModule.Contract.Inference(&_ILLamaModule.TransactOpts, prompt, seed, steps)
}

// Inference is a paid mutator transaction binding the contract method 0x3c8c648f.
//
// Solidity: function inference(string prompt, int64 seed, int32 steps) payable returns(string result)
func (_ILLamaModule *ILLamaModuleTransactorSession) Inference(prompt string, seed int64, steps int32) (*types.Transaction, error) {
	return _ILLamaModule.Contract.Inference(&_ILLamaModule.TransactOpts, prompt, seed, steps)
}

// ILLamaModuleInferenceResultIterator is returned from FilterInferenceResult and is used to iterate over the raw logs and unpacked data for InferenceResult events raised by the ILLamaModule contract.
type ILLamaModuleInferenceResultIterator struct {
	Event *ILLamaModuleInferenceResult // Event containing the contract specifics and raw log

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
func (it *ILLamaModuleInferenceResultIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ILLamaModuleInferenceResult)
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
		it.Event = new(ILLamaModuleInferenceResult)
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
func (it *ILLamaModuleInferenceResultIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ILLamaModuleInferenceResultIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ILLamaModuleInferenceResult represents a InferenceResult event raised by the ILLamaModule contract.
type ILLamaModuleInferenceResult struct {
	Result string
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterInferenceResult is a free log retrieval operation binding the contract event 0xa347bd043cd95f5d7359401ab21e8d4789094e7a5bb26535cd9790938a428e30.
//
// Solidity: event InferenceResult(string result)
func (_ILLamaModule *ILLamaModuleFilterer) FilterInferenceResult(opts *bind.FilterOpts) (*ILLamaModuleInferenceResultIterator, error) {

	logs, sub, err := _ILLamaModule.contract.FilterLogs(opts, "InferenceResult")
	if err != nil {
		return nil, err
	}
	return &ILLamaModuleInferenceResultIterator{contract: _ILLamaModule.contract, event: "InferenceResult", logs: logs, sub: sub}, nil
}

// WatchInferenceResult is a free log subscription operation binding the contract event 0xa347bd043cd95f5d7359401ab21e8d4789094e7a5bb26535cd9790938a428e30.
//
// Solidity: event InferenceResult(string result)
func (_ILLamaModule *ILLamaModuleFilterer) WatchInferenceResult(opts *bind.WatchOpts, sink chan<- *ILLamaModuleInferenceResult) (event.Subscription, error) {

	logs, sub, err := _ILLamaModule.contract.WatchLogs(opts, "InferenceResult")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ILLamaModuleInferenceResult)
				if err := _ILLamaModule.contract.UnpackLog(event, "InferenceResult", log); err != nil {
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

// ParseInferenceResult is a log parse operation binding the contract event 0xa347bd043cd95f5d7359401ab21e8d4789094e7a5bb26535cd9790938a428e30.
//
// Solidity: event InferenceResult(string result)
func (_ILLamaModule *ILLamaModuleFilterer) ParseInferenceResult(log types.Log) (*ILLamaModuleInferenceResult, error) {
	event := new(ILLamaModuleInferenceResult)
	if err := _ILLamaModule.contract.UnpackLog(event, "InferenceResult", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
