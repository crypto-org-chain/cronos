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

// RelayerFunctionsMetaData contains all meta data concerning the RelayerFunctions contract.
var RelayerFunctionsMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"acknowledgement\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelCloseConfirm\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelCloseInit\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelOpenAck\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelOpenConfirm\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelOpenInit\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelOpenTry\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"connectionOpenAck\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"connectionOpenConfirm\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"connectionOpenInit\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"connectionOpenTry\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"createClient\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"recvPacket\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"submitMisbehaviour\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"timeout\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"timeoutOnClose\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"updateClient\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"signer\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"upgradeClient\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// RelayerFunctionsABI is the input ABI used to generate the binding from.
// Deprecated: Use RelayerFunctionsMetaData.ABI instead.
var RelayerFunctionsABI = RelayerFunctionsMetaData.ABI

// RelayerFunctions is an auto generated Go binding around an Ethereum contract.
type RelayerFunctions struct {
	RelayerFunctionsCaller     // Read-only binding to the contract
	RelayerFunctionsTransactor // Write-only binding to the contract
	RelayerFunctionsFilterer   // Log filterer for contract events
}

// RelayerFunctionsCaller is an auto generated read-only Go binding around an Ethereum contract.
type RelayerFunctionsCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayerFunctionsTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RelayerFunctionsTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayerFunctionsFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RelayerFunctionsFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayerFunctionsSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RelayerFunctionsSession struct {
	Contract     *RelayerFunctions // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RelayerFunctionsCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RelayerFunctionsCallerSession struct {
	Contract *RelayerFunctionsCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// RelayerFunctionsTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RelayerFunctionsTransactorSession struct {
	Contract     *RelayerFunctionsTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// RelayerFunctionsRaw is an auto generated low-level Go binding around an Ethereum contract.
type RelayerFunctionsRaw struct {
	Contract *RelayerFunctions // Generic contract binding to access the raw methods on
}

// RelayerFunctionsCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RelayerFunctionsCallerRaw struct {
	Contract *RelayerFunctionsCaller // Generic read-only contract binding to access the raw methods on
}

// RelayerFunctionsTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RelayerFunctionsTransactorRaw struct {
	Contract *RelayerFunctionsTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRelayerFunctions creates a new instance of RelayerFunctions, bound to a specific deployed contract.
func NewRelayerFunctions(address common.Address, backend bind.ContractBackend) (*RelayerFunctions, error) {
	contract, err := bindRelayerFunctions(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RelayerFunctions{RelayerFunctionsCaller: RelayerFunctionsCaller{contract: contract}, RelayerFunctionsTransactor: RelayerFunctionsTransactor{contract: contract}, RelayerFunctionsFilterer: RelayerFunctionsFilterer{contract: contract}}, nil
}

// NewRelayerFunctionsCaller creates a new read-only instance of RelayerFunctions, bound to a specific deployed contract.
func NewRelayerFunctionsCaller(address common.Address, caller bind.ContractCaller) (*RelayerFunctionsCaller, error) {
	contract, err := bindRelayerFunctions(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RelayerFunctionsCaller{contract: contract}, nil
}

// NewRelayerFunctionsTransactor creates a new write-only instance of RelayerFunctions, bound to a specific deployed contract.
func NewRelayerFunctionsTransactor(address common.Address, transactor bind.ContractTransactor) (*RelayerFunctionsTransactor, error) {
	contract, err := bindRelayerFunctions(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RelayerFunctionsTransactor{contract: contract}, nil
}

// NewRelayerFunctionsFilterer creates a new log filterer instance of RelayerFunctions, bound to a specific deployed contract.
func NewRelayerFunctionsFilterer(address common.Address, filterer bind.ContractFilterer) (*RelayerFunctionsFilterer, error) {
	contract, err := bindRelayerFunctions(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RelayerFunctionsFilterer{contract: contract}, nil
}

// bindRelayerFunctions binds a generic wrapper to an already deployed contract.
func bindRelayerFunctions(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := RelayerFunctionsMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RelayerFunctions *RelayerFunctionsRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RelayerFunctions.Contract.RelayerFunctionsCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RelayerFunctions *RelayerFunctionsRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.RelayerFunctionsTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RelayerFunctions *RelayerFunctionsRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.RelayerFunctionsTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RelayerFunctions *RelayerFunctionsCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RelayerFunctions.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RelayerFunctions *RelayerFunctionsTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RelayerFunctions *RelayerFunctionsTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.contract.Transact(opts, method, params...)
}

// Acknowledgement is a paid mutator transaction binding the contract method 0x2dd03820.
//
// Solidity: function acknowledgement(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) Acknowledgement(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "acknowledgement", signer, data)
}

// Acknowledgement is a paid mutator transaction binding the contract method 0x2dd03820.
//
// Solidity: function acknowledgement(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) Acknowledgement(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.Acknowledgement(&_RelayerFunctions.TransactOpts, signer, data)
}

// Acknowledgement is a paid mutator transaction binding the contract method 0x2dd03820.
//
// Solidity: function acknowledgement(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) Acknowledgement(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.Acknowledgement(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelCloseConfirm is a paid mutator transaction binding the contract method 0xafde3b9c.
//
// Solidity: function channelCloseConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelCloseConfirm(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelCloseConfirm", signer, data)
}

// ChannelCloseConfirm is a paid mutator transaction binding the contract method 0xafde3b9c.
//
// Solidity: function channelCloseConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelCloseConfirm(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelCloseConfirm(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelCloseConfirm is a paid mutator transaction binding the contract method 0xafde3b9c.
//
// Solidity: function channelCloseConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelCloseConfirm(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelCloseConfirm(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelCloseInit is a paid mutator transaction binding the contract method 0x5108b479.
//
// Solidity: function channelCloseInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelCloseInit(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelCloseInit", signer, data)
}

// ChannelCloseInit is a paid mutator transaction binding the contract method 0x5108b479.
//
// Solidity: function channelCloseInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelCloseInit(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelCloseInit(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelCloseInit is a paid mutator transaction binding the contract method 0x5108b479.
//
// Solidity: function channelCloseInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelCloseInit(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelCloseInit(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelOpenAck is a paid mutator transaction binding the contract method 0xe66d0380.
//
// Solidity: function channelOpenAck(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelOpenAck(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelOpenAck", signer, data)
}

// ChannelOpenAck is a paid mutator transaction binding the contract method 0xe66d0380.
//
// Solidity: function channelOpenAck(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelOpenAck(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenAck(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelOpenAck is a paid mutator transaction binding the contract method 0xe66d0380.
//
// Solidity: function channelOpenAck(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelOpenAck(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenAck(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelOpenConfirm is a paid mutator transaction binding the contract method 0xc20e1316.
//
// Solidity: function channelOpenConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelOpenConfirm(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelOpenConfirm", signer, data)
}

// ChannelOpenConfirm is a paid mutator transaction binding the contract method 0xc20e1316.
//
// Solidity: function channelOpenConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelOpenConfirm(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenConfirm(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelOpenConfirm is a paid mutator transaction binding the contract method 0xc20e1316.
//
// Solidity: function channelOpenConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelOpenConfirm(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenConfirm(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelOpenInit is a paid mutator transaction binding the contract method 0x835e72b3.
//
// Solidity: function channelOpenInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelOpenInit(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelOpenInit", signer, data)
}

// ChannelOpenInit is a paid mutator transaction binding the contract method 0x835e72b3.
//
// Solidity: function channelOpenInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelOpenInit(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenInit(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelOpenInit is a paid mutator transaction binding the contract method 0x835e72b3.
//
// Solidity: function channelOpenInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelOpenInit(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenInit(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelOpenTry is a paid mutator transaction binding the contract method 0x0e5745cf.
//
// Solidity: function channelOpenTry(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelOpenTry(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelOpenTry", signer, data)
}

// ChannelOpenTry is a paid mutator transaction binding the contract method 0x0e5745cf.
//
// Solidity: function channelOpenTry(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelOpenTry(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenTry(&_RelayerFunctions.TransactOpts, signer, data)
}

// ChannelOpenTry is a paid mutator transaction binding the contract method 0x0e5745cf.
//
// Solidity: function channelOpenTry(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelOpenTry(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenTry(&_RelayerFunctions.TransactOpts, signer, data)
}

// ConnectionOpenAck is a paid mutator transaction binding the contract method 0x027868e8.
//
// Solidity: function connectionOpenAck(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ConnectionOpenAck(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "connectionOpenAck", signer, data)
}

// ConnectionOpenAck is a paid mutator transaction binding the contract method 0x027868e8.
//
// Solidity: function connectionOpenAck(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ConnectionOpenAck(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenAck(&_RelayerFunctions.TransactOpts, signer, data)
}

// ConnectionOpenAck is a paid mutator transaction binding the contract method 0x027868e8.
//
// Solidity: function connectionOpenAck(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ConnectionOpenAck(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenAck(&_RelayerFunctions.TransactOpts, signer, data)
}

// ConnectionOpenConfirm is a paid mutator transaction binding the contract method 0xcd281189.
//
// Solidity: function connectionOpenConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ConnectionOpenConfirm(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "connectionOpenConfirm", signer, data)
}

// ConnectionOpenConfirm is a paid mutator transaction binding the contract method 0xcd281189.
//
// Solidity: function connectionOpenConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ConnectionOpenConfirm(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenConfirm(&_RelayerFunctions.TransactOpts, signer, data)
}

// ConnectionOpenConfirm is a paid mutator transaction binding the contract method 0xcd281189.
//
// Solidity: function connectionOpenConfirm(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ConnectionOpenConfirm(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenConfirm(&_RelayerFunctions.TransactOpts, signer, data)
}

// ConnectionOpenInit is a paid mutator transaction binding the contract method 0x07fc7843.
//
// Solidity: function connectionOpenInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ConnectionOpenInit(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "connectionOpenInit", signer, data)
}

// ConnectionOpenInit is a paid mutator transaction binding the contract method 0x07fc7843.
//
// Solidity: function connectionOpenInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ConnectionOpenInit(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenInit(&_RelayerFunctions.TransactOpts, signer, data)
}

// ConnectionOpenInit is a paid mutator transaction binding the contract method 0x07fc7843.
//
// Solidity: function connectionOpenInit(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ConnectionOpenInit(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenInit(&_RelayerFunctions.TransactOpts, signer, data)
}

// ConnectionOpenTry is a paid mutator transaction binding the contract method 0xb4f69b9e.
//
// Solidity: function connectionOpenTry(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ConnectionOpenTry(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "connectionOpenTry", signer, data)
}

// ConnectionOpenTry is a paid mutator transaction binding the contract method 0xb4f69b9e.
//
// Solidity: function connectionOpenTry(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ConnectionOpenTry(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenTry(&_RelayerFunctions.TransactOpts, signer, data)
}

// ConnectionOpenTry is a paid mutator transaction binding the contract method 0xb4f69b9e.
//
// Solidity: function connectionOpenTry(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ConnectionOpenTry(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenTry(&_RelayerFunctions.TransactOpts, signer, data)
}

// CreateClient is a paid mutator transaction binding the contract method 0xbbcd46fd.
//
// Solidity: function createClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) CreateClient(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "createClient", signer, data)
}

// CreateClient is a paid mutator transaction binding the contract method 0xbbcd46fd.
//
// Solidity: function createClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) CreateClient(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.CreateClient(&_RelayerFunctions.TransactOpts, signer, data)
}

// CreateClient is a paid mutator transaction binding the contract method 0xbbcd46fd.
//
// Solidity: function createClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) CreateClient(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.CreateClient(&_RelayerFunctions.TransactOpts, signer, data)
}

// RecvPacket is a paid mutator transaction binding the contract method 0x8faf3716.
//
// Solidity: function recvPacket(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) RecvPacket(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "recvPacket", signer, data)
}

// RecvPacket is a paid mutator transaction binding the contract method 0x8faf3716.
//
// Solidity: function recvPacket(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) RecvPacket(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.RecvPacket(&_RelayerFunctions.TransactOpts, signer, data)
}

// RecvPacket is a paid mutator transaction binding the contract method 0x8faf3716.
//
// Solidity: function recvPacket(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) RecvPacket(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.RecvPacket(&_RelayerFunctions.TransactOpts, signer, data)
}

// SubmitMisbehaviour is a paid mutator transaction binding the contract method 0xd4461718.
//
// Solidity: function submitMisbehaviour(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) SubmitMisbehaviour(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "submitMisbehaviour", signer, data)
}

// SubmitMisbehaviour is a paid mutator transaction binding the contract method 0xd4461718.
//
// Solidity: function submitMisbehaviour(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) SubmitMisbehaviour(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.SubmitMisbehaviour(&_RelayerFunctions.TransactOpts, signer, data)
}

// SubmitMisbehaviour is a paid mutator transaction binding the contract method 0xd4461718.
//
// Solidity: function submitMisbehaviour(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) SubmitMisbehaviour(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.SubmitMisbehaviour(&_RelayerFunctions.TransactOpts, signer, data)
}

// Timeout is a paid mutator transaction binding the contract method 0x8ba86f29.
//
// Solidity: function timeout(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) Timeout(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "timeout", signer, data)
}

// Timeout is a paid mutator transaction binding the contract method 0x8ba86f29.
//
// Solidity: function timeout(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) Timeout(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.Timeout(&_RelayerFunctions.TransactOpts, signer, data)
}

// Timeout is a paid mutator transaction binding the contract method 0x8ba86f29.
//
// Solidity: function timeout(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) Timeout(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.Timeout(&_RelayerFunctions.TransactOpts, signer, data)
}

// TimeoutOnClose is a paid mutator transaction binding the contract method 0x0546845a.
//
// Solidity: function timeoutOnClose(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) TimeoutOnClose(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "timeoutOnClose", signer, data)
}

// TimeoutOnClose is a paid mutator transaction binding the contract method 0x0546845a.
//
// Solidity: function timeoutOnClose(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) TimeoutOnClose(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.TimeoutOnClose(&_RelayerFunctions.TransactOpts, signer, data)
}

// TimeoutOnClose is a paid mutator transaction binding the contract method 0x0546845a.
//
// Solidity: function timeoutOnClose(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) TimeoutOnClose(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.TimeoutOnClose(&_RelayerFunctions.TransactOpts, signer, data)
}

// UpdateClient is a paid mutator transaction binding the contract method 0x8b6789fd.
//
// Solidity: function updateClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClient(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClient", signer, data)
}

// UpdateClient is a paid mutator transaction binding the contract method 0x8b6789fd.
//
// Solidity: function updateClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClient(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClient(&_RelayerFunctions.TransactOpts, signer, data)
}

// UpdateClient is a paid mutator transaction binding the contract method 0x8b6789fd.
//
// Solidity: function updateClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClient(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClient(&_RelayerFunctions.TransactOpts, signer, data)
}

// UpgradeClient is a paid mutator transaction binding the contract method 0x81909f7c.
//
// Solidity: function upgradeClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpgradeClient(opts *bind.TransactOpts, signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "upgradeClient", signer, data)
}

// UpgradeClient is a paid mutator transaction binding the contract method 0x81909f7c.
//
// Solidity: function upgradeClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) UpgradeClient(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpgradeClient(&_RelayerFunctions.TransactOpts, signer, data)
}

// UpgradeClient is a paid mutator transaction binding the contract method 0x81909f7c.
//
// Solidity: function upgradeClient(address signer, bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpgradeClient(signer common.Address, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpgradeClient(&_RelayerFunctions.TransactOpts, signer, data)
}
