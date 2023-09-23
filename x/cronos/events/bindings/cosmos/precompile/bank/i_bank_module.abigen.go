// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bank

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

// BankModuleMetaData contains all meta data concerning the BankModule contract.
var BankModuleMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"burn\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"mint\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// BankModuleABI is the input ABI used to generate the binding from.
// Deprecated: Use BankModuleMetaData.ABI instead.
var BankModuleABI = BankModuleMetaData.ABI

// BankModule is an auto generated Go binding around an Ethereum contract.
type BankModule struct {
	BankModuleCaller     // Read-only binding to the contract
	BankModuleTransactor // Write-only binding to the contract
	BankModuleFilterer   // Log filterer for contract events
}

// BankModuleCaller is an auto generated read-only Go binding around an Ethereum contract.
type BankModuleCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BankModuleTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BankModuleTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BankModuleFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BankModuleFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BankModuleSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BankModuleSession struct {
	Contract     *BankModule       // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BankModuleCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BankModuleCallerSession struct {
	Contract *BankModuleCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts     // Call options to use throughout this session
}

// BankModuleTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BankModuleTransactorSession struct {
	Contract     *BankModuleTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// BankModuleRaw is an auto generated low-level Go binding around an Ethereum contract.
type BankModuleRaw struct {
	Contract *BankModule // Generic contract binding to access the raw methods on
}

// BankModuleCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BankModuleCallerRaw struct {
	Contract *BankModuleCaller // Generic read-only contract binding to access the raw methods on
}

// BankModuleTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BankModuleTransactorRaw struct {
	Contract *BankModuleTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBankModule creates a new instance of BankModule, bound to a specific deployed contract.
func NewBankModule(address common.Address, backend bind.ContractBackend) (*BankModule, error) {
	contract, err := bindBankModule(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BankModule{BankModuleCaller: BankModuleCaller{contract: contract}, BankModuleTransactor: BankModuleTransactor{contract: contract}, BankModuleFilterer: BankModuleFilterer{contract: contract}}, nil
}

// NewBankModuleCaller creates a new read-only instance of BankModule, bound to a specific deployed contract.
func NewBankModuleCaller(address common.Address, caller bind.ContractCaller) (*BankModuleCaller, error) {
	contract, err := bindBankModule(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BankModuleCaller{contract: contract}, nil
}

// NewBankModuleTransactor creates a new write-only instance of BankModule, bound to a specific deployed contract.
func NewBankModuleTransactor(address common.Address, transactor bind.ContractTransactor) (*BankModuleTransactor, error) {
	contract, err := bindBankModule(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BankModuleTransactor{contract: contract}, nil
}

// NewBankModuleFilterer creates a new log filterer instance of BankModule, bound to a specific deployed contract.
func NewBankModuleFilterer(address common.Address, filterer bind.ContractFilterer) (*BankModuleFilterer, error) {
	contract, err := bindBankModule(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BankModuleFilterer{contract: contract}, nil
}

// bindBankModule binds a generic wrapper to an already deployed contract.
func bindBankModule(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(BankModuleABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BankModule *BankModuleRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BankModule.Contract.BankModuleCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BankModule *BankModuleRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BankModule.Contract.BankModuleTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BankModule *BankModuleRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BankModule.Contract.BankModuleTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BankModule *BankModuleCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BankModule.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BankModule *BankModuleTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BankModule.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BankModule *BankModuleTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BankModule.Contract.contract.Transact(opts, method, params...)
}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address , address ) view returns(uint256)
func (_BankModule *BankModuleCaller) BalanceOf(opts *bind.CallOpts, arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _BankModule.contract.Call(opts, &out, "balanceOf", arg0, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address , address ) view returns(uint256)
func (_BankModule *BankModuleSession) BalanceOf(arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	return _BankModule.Contract.BalanceOf(&_BankModule.CallOpts, arg0, arg1)
}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address , address ) view returns(uint256)
func (_BankModule *BankModuleCallerSession) BalanceOf(arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	return _BankModule.Contract.BalanceOf(&_BankModule.CallOpts, arg0, arg1)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleTransactor) Burn(opts *bind.TransactOpts, arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _BankModule.contract.Transact(opts, "burn", arg0, arg1)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleSession) Burn(arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _BankModule.Contract.Burn(&_BankModule.TransactOpts, arg0, arg1)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleTransactorSession) Burn(arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _BankModule.Contract.Burn(&_BankModule.TransactOpts, arg0, arg1)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleTransactor) Mint(opts *bind.TransactOpts, arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _BankModule.contract.Transact(opts, "mint", arg0, arg1)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleSession) Mint(arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _BankModule.Contract.Mint(&_BankModule.TransactOpts, arg0, arg1)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleTransactorSession) Mint(arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _BankModule.Contract.Mint(&_BankModule.TransactOpts, arg0, arg1)
}

// Transfer is a paid mutator transaction binding the contract method 0xbeabacc8.
//
// Solidity: function transfer(address , address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleTransactor) Transfer(opts *bind.TransactOpts, arg0 common.Address, arg1 common.Address, arg2 *big.Int) (*types.Transaction, error) {
	return _BankModule.contract.Transact(opts, "transfer", arg0, arg1, arg2)
}

// Transfer is a paid mutator transaction binding the contract method 0xbeabacc8.
//
// Solidity: function transfer(address , address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleSession) Transfer(arg0 common.Address, arg1 common.Address, arg2 *big.Int) (*types.Transaction, error) {
	return _BankModule.Contract.Transfer(&_BankModule.TransactOpts, arg0, arg1, arg2)
}

// Transfer is a paid mutator transaction binding the contract method 0xbeabacc8.
//
// Solidity: function transfer(address , address , uint256 ) payable returns(bool)
func (_BankModule *BankModuleTransactorSession) Transfer(arg0 common.Address, arg1 common.Address, arg2 *big.Int) (*types.Transaction, error) {
	return _BankModule.Contract.Transfer(&_BankModule.TransactOpts, arg0, arg1, arg2)
}
