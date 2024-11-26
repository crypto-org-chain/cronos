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
	ABI: "[{\"inputs\":[{\"internalType\":\"string\",\"name\":\"prompt\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"temperature\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"steps\",\"type\":\"uint256\"}],\"name\":\"inference\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"result\",\"type\":\"string\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
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

// Inference is a paid mutator transaction binding the contract method 0x14cf839b.
//
// Solidity: function inference(string prompt, uint256 temperature, uint256 seed, uint256 steps) payable returns(string result)
func (_ILLamaModule *ILLamaModuleTransactor) Inference(opts *bind.TransactOpts, prompt string, temperature *big.Int, seed *big.Int, steps *big.Int) (*types.Transaction, error) {
	return _ILLamaModule.contract.Transact(opts, "inference", prompt, temperature, seed, steps)
}

// Inference is a paid mutator transaction binding the contract method 0x14cf839b.
//
// Solidity: function inference(string prompt, uint256 temperature, uint256 seed, uint256 steps) payable returns(string result)
func (_ILLamaModule *ILLamaModuleSession) Inference(prompt string, temperature *big.Int, seed *big.Int, steps *big.Int) (*types.Transaction, error) {
	return _ILLamaModule.Contract.Inference(&_ILLamaModule.TransactOpts, prompt, temperature, seed, steps)
}

// Inference is a paid mutator transaction binding the contract method 0x14cf839b.
//
// Solidity: function inference(string prompt, uint256 temperature, uint256 seed, uint256 steps) payable returns(string result)
func (_ILLamaModule *ILLamaModuleTransactorSession) Inference(prompt string, temperature *big.Int, seed *big.Int, steps *big.Int) (*types.Transaction, error) {
	return _ILLamaModule.Contract.Inference(&_ILLamaModule.TransactOpts, prompt, temperature, seed, steps)
}
