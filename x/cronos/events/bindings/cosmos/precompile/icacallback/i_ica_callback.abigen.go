// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package icacallback

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

// ICACallbackMetaData contains all meta data concerning the ICACallback contract.
var ICACallbackMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"string\",\"name\":\"packetSrcChannel\",\"type\":\"string\"},{\"internalType\":\"uint64\",\"name\":\"seq\",\"type\":\"uint64\"},{\"internalType\":\"bool\",\"name\":\"ack\",\"type\":\"bool\"}],\"name\":\"onPacketResultCallback\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// ICACallbackABI is the input ABI used to generate the binding from.
// Deprecated: Use ICACallbackMetaData.ABI instead.
var ICACallbackABI = ICACallbackMetaData.ABI

// ICACallback is an auto generated Go binding around an Ethereum contract.
type ICACallback struct {
	ICACallbackCaller     // Read-only binding to the contract
	ICACallbackTransactor // Write-only binding to the contract
	ICACallbackFilterer   // Log filterer for contract events
}

// ICACallbackCaller is an auto generated read-only Go binding around an Ethereum contract.
type ICACallbackCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ICACallbackTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ICACallbackTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ICACallbackFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ICACallbackFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ICACallbackSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ICACallbackSession struct {
	Contract     *ICACallback      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ICACallbackCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ICACallbackCallerSession struct {
	Contract *ICACallbackCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// ICACallbackTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ICACallbackTransactorSession struct {
	Contract     *ICACallbackTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// ICACallbackRaw is an auto generated low-level Go binding around an Ethereum contract.
type ICACallbackRaw struct {
	Contract *ICACallback // Generic contract binding to access the raw methods on
}

// ICACallbackCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ICACallbackCallerRaw struct {
	Contract *ICACallbackCaller // Generic read-only contract binding to access the raw methods on
}

// ICACallbackTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ICACallbackTransactorRaw struct {
	Contract *ICACallbackTransactor // Generic write-only contract binding to access the raw methods on
}

// NewICACallback creates a new instance of ICACallback, bound to a specific deployed contract.
func NewICACallback(address common.Address, backend bind.ContractBackend) (*ICACallback, error) {
	contract, err := bindICACallback(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ICACallback{ICACallbackCaller: ICACallbackCaller{contract: contract}, ICACallbackTransactor: ICACallbackTransactor{contract: contract}, ICACallbackFilterer: ICACallbackFilterer{contract: contract}}, nil
}

// NewICACallbackCaller creates a new read-only instance of ICACallback, bound to a specific deployed contract.
func NewICACallbackCaller(address common.Address, caller bind.ContractCaller) (*ICACallbackCaller, error) {
	contract, err := bindICACallback(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ICACallbackCaller{contract: contract}, nil
}

// NewICACallbackTransactor creates a new write-only instance of ICACallback, bound to a specific deployed contract.
func NewICACallbackTransactor(address common.Address, transactor bind.ContractTransactor) (*ICACallbackTransactor, error) {
	contract, err := bindICACallback(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ICACallbackTransactor{contract: contract}, nil
}

// NewICACallbackFilterer creates a new log filterer instance of ICACallback, bound to a specific deployed contract.
func NewICACallbackFilterer(address common.Address, filterer bind.ContractFilterer) (*ICACallbackFilterer, error) {
	contract, err := bindICACallback(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ICACallbackFilterer{contract: contract}, nil
}

// bindICACallback binds a generic wrapper to an already deployed contract.
func bindICACallback(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ICACallbackABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ICACallback *ICACallbackRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ICACallback.Contract.ICACallbackCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ICACallback *ICACallbackRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ICACallback.Contract.ICACallbackTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ICACallback *ICACallbackRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ICACallback.Contract.ICACallbackTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ICACallback *ICACallbackCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ICACallback.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ICACallback *ICACallbackTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ICACallback.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ICACallback *ICACallbackTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ICACallback.Contract.contract.Transact(opts, method, params...)
}

// OnPacketResultCallback is a paid mutator transaction binding the contract method 0xd2712162.
//
// Solidity: function onPacketResultCallback(string packetSrcChannel, uint64 seq, bool ack) payable returns(bool)
func (_ICACallback *ICACallbackTransactor) OnPacketResultCallback(opts *bind.TransactOpts, packetSrcChannel string, seq uint64, ack bool) (*types.Transaction, error) {
	return _ICACallback.contract.Transact(opts, "onPacketResultCallback", packetSrcChannel, seq, ack)
}

// OnPacketResultCallback is a paid mutator transaction binding the contract method 0xd2712162.
//
// Solidity: function onPacketResultCallback(string packetSrcChannel, uint64 seq, bool ack) payable returns(bool)
func (_ICACallback *ICACallbackSession) OnPacketResultCallback(packetSrcChannel string, seq uint64, ack bool) (*types.Transaction, error) {
	return _ICACallback.Contract.OnPacketResultCallback(&_ICACallback.TransactOpts, packetSrcChannel, seq, ack)
}

// OnPacketResultCallback is a paid mutator transaction binding the contract method 0xd2712162.
//
// Solidity: function onPacketResultCallback(string packetSrcChannel, uint64 seq, bool ack) payable returns(bool)
func (_ICACallback *ICACallbackTransactorSession) OnPacketResultCallback(packetSrcChannel string, seq uint64, ack bool) (*types.Transaction, error) {
	return _ICACallback.Contract.OnPacketResultCallback(&_ICACallback.TransactOpts, packetSrcChannel, seq, ack)
}
