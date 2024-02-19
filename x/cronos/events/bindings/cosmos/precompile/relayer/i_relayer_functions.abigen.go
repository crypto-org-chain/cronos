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
	ABI: "[{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"acknowledgement\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelCloseConfirm\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelCloseInit\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelOpenAck\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelOpenConfirm\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelOpenInit\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"channelOpenTry\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"connectionOpenAck\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"connectionOpenConfirm\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"connectionOpenInit\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"connectionOpenTry\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"createClient\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"recvPacket\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"submitMisbehaviour\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"timeout\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"timeoutOnClose\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"updateClient\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndAcknowledgement\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndChannelCloseConfirm\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndChannelCloseInit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndChannelOpenAck\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndChannelOpenConfirm\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndChannelOpenInit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndChannelOpenTry\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndConnectionOpenAck\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndConnectionOpenConfirm\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndConnectionOpenInit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndConnectionOpenTry\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndRecvPacket\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data1\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"data2\",\"type\":\"bytes\"}],\"name\":\"updateClientAndTimeout\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"upgradeClient\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
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

// Acknowledgement is a paid mutator transaction binding the contract method 0x07ed2b37.
//
// Solidity: function acknowledgement(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) Acknowledgement(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "acknowledgement", data)
}

// Acknowledgement is a paid mutator transaction binding the contract method 0x07ed2b37.
//
// Solidity: function acknowledgement(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) Acknowledgement(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.Acknowledgement(&_RelayerFunctions.TransactOpts, data)
}

// Acknowledgement is a paid mutator transaction binding the contract method 0x07ed2b37.
//
// Solidity: function acknowledgement(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) Acknowledgement(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.Acknowledgement(&_RelayerFunctions.TransactOpts, data)
}

// ChannelCloseConfirm is a paid mutator transaction binding the contract method 0xc9741674.
//
// Solidity: function channelCloseConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelCloseConfirm(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelCloseConfirm", data)
}

// ChannelCloseConfirm is a paid mutator transaction binding the contract method 0xc9741674.
//
// Solidity: function channelCloseConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelCloseConfirm(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelCloseConfirm(&_RelayerFunctions.TransactOpts, data)
}

// ChannelCloseConfirm is a paid mutator transaction binding the contract method 0xc9741674.
//
// Solidity: function channelCloseConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelCloseConfirm(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelCloseConfirm(&_RelayerFunctions.TransactOpts, data)
}

// ChannelCloseInit is a paid mutator transaction binding the contract method 0x44ba8a17.
//
// Solidity: function channelCloseInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelCloseInit(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelCloseInit", data)
}

// ChannelCloseInit is a paid mutator transaction binding the contract method 0x44ba8a17.
//
// Solidity: function channelCloseInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelCloseInit(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelCloseInit(&_RelayerFunctions.TransactOpts, data)
}

// ChannelCloseInit is a paid mutator transaction binding the contract method 0x44ba8a17.
//
// Solidity: function channelCloseInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelCloseInit(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelCloseInit(&_RelayerFunctions.TransactOpts, data)
}

// ChannelOpenAck is a paid mutator transaction binding the contract method 0xd859b9f4.
//
// Solidity: function channelOpenAck(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelOpenAck(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelOpenAck", data)
}

// ChannelOpenAck is a paid mutator transaction binding the contract method 0xd859b9f4.
//
// Solidity: function channelOpenAck(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelOpenAck(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenAck(&_RelayerFunctions.TransactOpts, data)
}

// ChannelOpenAck is a paid mutator transaction binding the contract method 0xd859b9f4.
//
// Solidity: function channelOpenAck(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelOpenAck(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenAck(&_RelayerFunctions.TransactOpts, data)
}

// ChannelOpenConfirm is a paid mutator transaction binding the contract method 0x5e1fad7d.
//
// Solidity: function channelOpenConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelOpenConfirm(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelOpenConfirm", data)
}

// ChannelOpenConfirm is a paid mutator transaction binding the contract method 0x5e1fad7d.
//
// Solidity: function channelOpenConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelOpenConfirm(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenConfirm(&_RelayerFunctions.TransactOpts, data)
}

// ChannelOpenConfirm is a paid mutator transaction binding the contract method 0x5e1fad7d.
//
// Solidity: function channelOpenConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelOpenConfirm(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenConfirm(&_RelayerFunctions.TransactOpts, data)
}

// ChannelOpenInit is a paid mutator transaction binding the contract method 0x63d2dc06.
//
// Solidity: function channelOpenInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelOpenInit(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelOpenInit", data)
}

// ChannelOpenInit is a paid mutator transaction binding the contract method 0x63d2dc06.
//
// Solidity: function channelOpenInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelOpenInit(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenInit(&_RelayerFunctions.TransactOpts, data)
}

// ChannelOpenInit is a paid mutator transaction binding the contract method 0x63d2dc06.
//
// Solidity: function channelOpenInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelOpenInit(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenInit(&_RelayerFunctions.TransactOpts, data)
}

// ChannelOpenTry is a paid mutator transaction binding the contract method 0xf45b605e.
//
// Solidity: function channelOpenTry(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ChannelOpenTry(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "channelOpenTry", data)
}

// ChannelOpenTry is a paid mutator transaction binding the contract method 0xf45b605e.
//
// Solidity: function channelOpenTry(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ChannelOpenTry(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenTry(&_RelayerFunctions.TransactOpts, data)
}

// ChannelOpenTry is a paid mutator transaction binding the contract method 0xf45b605e.
//
// Solidity: function channelOpenTry(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ChannelOpenTry(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ChannelOpenTry(&_RelayerFunctions.TransactOpts, data)
}

// ConnectionOpenAck is a paid mutator transaction binding the contract method 0xe9984826.
//
// Solidity: function connectionOpenAck(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ConnectionOpenAck(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "connectionOpenAck", data)
}

// ConnectionOpenAck is a paid mutator transaction binding the contract method 0xe9984826.
//
// Solidity: function connectionOpenAck(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ConnectionOpenAck(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenAck(&_RelayerFunctions.TransactOpts, data)
}

// ConnectionOpenAck is a paid mutator transaction binding the contract method 0xe9984826.
//
// Solidity: function connectionOpenAck(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ConnectionOpenAck(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenAck(&_RelayerFunctions.TransactOpts, data)
}

// ConnectionOpenConfirm is a paid mutator transaction binding the contract method 0xb710bcf2.
//
// Solidity: function connectionOpenConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ConnectionOpenConfirm(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "connectionOpenConfirm", data)
}

// ConnectionOpenConfirm is a paid mutator transaction binding the contract method 0xb710bcf2.
//
// Solidity: function connectionOpenConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ConnectionOpenConfirm(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenConfirm(&_RelayerFunctions.TransactOpts, data)
}

// ConnectionOpenConfirm is a paid mutator transaction binding the contract method 0xb710bcf2.
//
// Solidity: function connectionOpenConfirm(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ConnectionOpenConfirm(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenConfirm(&_RelayerFunctions.TransactOpts, data)
}

// ConnectionOpenInit is a paid mutator transaction binding the contract method 0x528e6644.
//
// Solidity: function connectionOpenInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ConnectionOpenInit(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "connectionOpenInit", data)
}

// ConnectionOpenInit is a paid mutator transaction binding the contract method 0x528e6644.
//
// Solidity: function connectionOpenInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ConnectionOpenInit(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenInit(&_RelayerFunctions.TransactOpts, data)
}

// ConnectionOpenInit is a paid mutator transaction binding the contract method 0x528e6644.
//
// Solidity: function connectionOpenInit(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ConnectionOpenInit(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenInit(&_RelayerFunctions.TransactOpts, data)
}

// ConnectionOpenTry is a paid mutator transaction binding the contract method 0x986fa270.
//
// Solidity: function connectionOpenTry(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) ConnectionOpenTry(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "connectionOpenTry", data)
}

// ConnectionOpenTry is a paid mutator transaction binding the contract method 0x986fa270.
//
// Solidity: function connectionOpenTry(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) ConnectionOpenTry(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenTry(&_RelayerFunctions.TransactOpts, data)
}

// ConnectionOpenTry is a paid mutator transaction binding the contract method 0x986fa270.
//
// Solidity: function connectionOpenTry(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) ConnectionOpenTry(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.ConnectionOpenTry(&_RelayerFunctions.TransactOpts, data)
}

// CreateClient is a paid mutator transaction binding the contract method 0x3df83afa.
//
// Solidity: function createClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) CreateClient(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "createClient", data)
}

// CreateClient is a paid mutator transaction binding the contract method 0x3df83afa.
//
// Solidity: function createClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) CreateClient(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.CreateClient(&_RelayerFunctions.TransactOpts, data)
}

// CreateClient is a paid mutator transaction binding the contract method 0x3df83afa.
//
// Solidity: function createClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) CreateClient(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.CreateClient(&_RelayerFunctions.TransactOpts, data)
}

// RecvPacket is a paid mutator transaction binding the contract method 0xf6a1539d.
//
// Solidity: function recvPacket(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) RecvPacket(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "recvPacket", data)
}

// RecvPacket is a paid mutator transaction binding the contract method 0xf6a1539d.
//
// Solidity: function recvPacket(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) RecvPacket(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.RecvPacket(&_RelayerFunctions.TransactOpts, data)
}

// RecvPacket is a paid mutator transaction binding the contract method 0xf6a1539d.
//
// Solidity: function recvPacket(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) RecvPacket(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.RecvPacket(&_RelayerFunctions.TransactOpts, data)
}

// SubmitMisbehaviour is a paid mutator transaction binding the contract method 0xa53b1c82.
//
// Solidity: function submitMisbehaviour(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) SubmitMisbehaviour(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "submitMisbehaviour", data)
}

// SubmitMisbehaviour is a paid mutator transaction binding the contract method 0xa53b1c82.
//
// Solidity: function submitMisbehaviour(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) SubmitMisbehaviour(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.SubmitMisbehaviour(&_RelayerFunctions.TransactOpts, data)
}

// SubmitMisbehaviour is a paid mutator transaction binding the contract method 0xa53b1c82.
//
// Solidity: function submitMisbehaviour(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) SubmitMisbehaviour(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.SubmitMisbehaviour(&_RelayerFunctions.TransactOpts, data)
}

// Timeout is a paid mutator transaction binding the contract method 0x6d2a27f6.
//
// Solidity: function timeout(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) Timeout(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "timeout", data)
}

// Timeout is a paid mutator transaction binding the contract method 0x6d2a27f6.
//
// Solidity: function timeout(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) Timeout(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.Timeout(&_RelayerFunctions.TransactOpts, data)
}

// Timeout is a paid mutator transaction binding the contract method 0x6d2a27f6.
//
// Solidity: function timeout(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) Timeout(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.Timeout(&_RelayerFunctions.TransactOpts, data)
}

// TimeoutOnClose is a paid mutator transaction binding the contract method 0x08f5d079.
//
// Solidity: function timeoutOnClose(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) TimeoutOnClose(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "timeoutOnClose", data)
}

// TimeoutOnClose is a paid mutator transaction binding the contract method 0x08f5d079.
//
// Solidity: function timeoutOnClose(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) TimeoutOnClose(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.TimeoutOnClose(&_RelayerFunctions.TransactOpts, data)
}

// TimeoutOnClose is a paid mutator transaction binding the contract method 0x08f5d079.
//
// Solidity: function timeoutOnClose(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) TimeoutOnClose(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.TimeoutOnClose(&_RelayerFunctions.TransactOpts, data)
}

// UpdateClient is a paid mutator transaction binding the contract method 0x0bece356.
//
// Solidity: function updateClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClient(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClient", data)
}

// UpdateClient is a paid mutator transaction binding the contract method 0x0bece356.
//
// Solidity: function updateClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClient(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClient(&_RelayerFunctions.TransactOpts, data)
}

// UpdateClient is a paid mutator transaction binding the contract method 0x0bece356.
//
// Solidity: function updateClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClient(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClient(&_RelayerFunctions.TransactOpts, data)
}

// UpdateClientAndAcknowledgement is a paid mutator transaction binding the contract method 0x65a939c6.
//
// Solidity: function updateClientAndAcknowledgement(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndAcknowledgement(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndAcknowledgement", data1, data2)
}

// UpdateClientAndAcknowledgement is a paid mutator transaction binding the contract method 0x65a939c6.
//
// Solidity: function updateClientAndAcknowledgement(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndAcknowledgement(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndAcknowledgement(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndAcknowledgement is a paid mutator transaction binding the contract method 0x65a939c6.
//
// Solidity: function updateClientAndAcknowledgement(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndAcknowledgement(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndAcknowledgement(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelCloseConfirm is a paid mutator transaction binding the contract method 0x9bbcbfd2.
//
// Solidity: function updateClientAndChannelCloseConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndChannelCloseConfirm(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndChannelCloseConfirm", data1, data2)
}

// UpdateClientAndChannelCloseConfirm is a paid mutator transaction binding the contract method 0x9bbcbfd2.
//
// Solidity: function updateClientAndChannelCloseConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndChannelCloseConfirm(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelCloseConfirm(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelCloseConfirm is a paid mutator transaction binding the contract method 0x9bbcbfd2.
//
// Solidity: function updateClientAndChannelCloseConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndChannelCloseConfirm(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelCloseConfirm(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelCloseInit is a paid mutator transaction binding the contract method 0x5447448d.
//
// Solidity: function updateClientAndChannelCloseInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndChannelCloseInit(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndChannelCloseInit", data1, data2)
}

// UpdateClientAndChannelCloseInit is a paid mutator transaction binding the contract method 0x5447448d.
//
// Solidity: function updateClientAndChannelCloseInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndChannelCloseInit(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelCloseInit(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelCloseInit is a paid mutator transaction binding the contract method 0x5447448d.
//
// Solidity: function updateClientAndChannelCloseInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndChannelCloseInit(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelCloseInit(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelOpenAck is a paid mutator transaction binding the contract method 0xc518ffc8.
//
// Solidity: function updateClientAndChannelOpenAck(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndChannelOpenAck(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndChannelOpenAck", data1, data2)
}

// UpdateClientAndChannelOpenAck is a paid mutator transaction binding the contract method 0xc518ffc8.
//
// Solidity: function updateClientAndChannelOpenAck(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndChannelOpenAck(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelOpenAck(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelOpenAck is a paid mutator transaction binding the contract method 0xc518ffc8.
//
// Solidity: function updateClientAndChannelOpenAck(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndChannelOpenAck(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelOpenAck(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelOpenConfirm is a paid mutator transaction binding the contract method 0x0982b806.
//
// Solidity: function updateClientAndChannelOpenConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndChannelOpenConfirm(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndChannelOpenConfirm", data1, data2)
}

// UpdateClientAndChannelOpenConfirm is a paid mutator transaction binding the contract method 0x0982b806.
//
// Solidity: function updateClientAndChannelOpenConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndChannelOpenConfirm(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelOpenConfirm(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelOpenConfirm is a paid mutator transaction binding the contract method 0x0982b806.
//
// Solidity: function updateClientAndChannelOpenConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndChannelOpenConfirm(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelOpenConfirm(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelOpenInit is a paid mutator transaction binding the contract method 0x66365fc4.
//
// Solidity: function updateClientAndChannelOpenInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndChannelOpenInit(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndChannelOpenInit", data1, data2)
}

// UpdateClientAndChannelOpenInit is a paid mutator transaction binding the contract method 0x66365fc4.
//
// Solidity: function updateClientAndChannelOpenInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndChannelOpenInit(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelOpenInit(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelOpenInit is a paid mutator transaction binding the contract method 0x66365fc4.
//
// Solidity: function updateClientAndChannelOpenInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndChannelOpenInit(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelOpenInit(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelOpenTry is a paid mutator transaction binding the contract method 0x33978088.
//
// Solidity: function updateClientAndChannelOpenTry(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndChannelOpenTry(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndChannelOpenTry", data1, data2)
}

// UpdateClientAndChannelOpenTry is a paid mutator transaction binding the contract method 0x33978088.
//
// Solidity: function updateClientAndChannelOpenTry(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndChannelOpenTry(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelOpenTry(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndChannelOpenTry is a paid mutator transaction binding the contract method 0x33978088.
//
// Solidity: function updateClientAndChannelOpenTry(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndChannelOpenTry(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndChannelOpenTry(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndConnectionOpenAck is a paid mutator transaction binding the contract method 0xfedb9353.
//
// Solidity: function updateClientAndConnectionOpenAck(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndConnectionOpenAck(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndConnectionOpenAck", data1, data2)
}

// UpdateClientAndConnectionOpenAck is a paid mutator transaction binding the contract method 0xfedb9353.
//
// Solidity: function updateClientAndConnectionOpenAck(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndConnectionOpenAck(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndConnectionOpenAck(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndConnectionOpenAck is a paid mutator transaction binding the contract method 0xfedb9353.
//
// Solidity: function updateClientAndConnectionOpenAck(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndConnectionOpenAck(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndConnectionOpenAck(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndConnectionOpenConfirm is a paid mutator transaction binding the contract method 0x70009dfc.
//
// Solidity: function updateClientAndConnectionOpenConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndConnectionOpenConfirm(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndConnectionOpenConfirm", data1, data2)
}

// UpdateClientAndConnectionOpenConfirm is a paid mutator transaction binding the contract method 0x70009dfc.
//
// Solidity: function updateClientAndConnectionOpenConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndConnectionOpenConfirm(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndConnectionOpenConfirm(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndConnectionOpenConfirm is a paid mutator transaction binding the contract method 0x70009dfc.
//
// Solidity: function updateClientAndConnectionOpenConfirm(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndConnectionOpenConfirm(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndConnectionOpenConfirm(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndConnectionOpenInit is a paid mutator transaction binding the contract method 0x491e69c7.
//
// Solidity: function updateClientAndConnectionOpenInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndConnectionOpenInit(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndConnectionOpenInit", data1, data2)
}

// UpdateClientAndConnectionOpenInit is a paid mutator transaction binding the contract method 0x491e69c7.
//
// Solidity: function updateClientAndConnectionOpenInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndConnectionOpenInit(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndConnectionOpenInit(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndConnectionOpenInit is a paid mutator transaction binding the contract method 0x491e69c7.
//
// Solidity: function updateClientAndConnectionOpenInit(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndConnectionOpenInit(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndConnectionOpenInit(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndConnectionOpenTry is a paid mutator transaction binding the contract method 0x5f3a7169.
//
// Solidity: function updateClientAndConnectionOpenTry(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndConnectionOpenTry(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndConnectionOpenTry", data1, data2)
}

// UpdateClientAndConnectionOpenTry is a paid mutator transaction binding the contract method 0x5f3a7169.
//
// Solidity: function updateClientAndConnectionOpenTry(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndConnectionOpenTry(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndConnectionOpenTry(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndConnectionOpenTry is a paid mutator transaction binding the contract method 0x5f3a7169.
//
// Solidity: function updateClientAndConnectionOpenTry(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndConnectionOpenTry(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndConnectionOpenTry(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndRecvPacket is a paid mutator transaction binding the contract method 0xd3cffc28.
//
// Solidity: function updateClientAndRecvPacket(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndRecvPacket(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndRecvPacket", data1, data2)
}

// UpdateClientAndRecvPacket is a paid mutator transaction binding the contract method 0xd3cffc28.
//
// Solidity: function updateClientAndRecvPacket(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndRecvPacket(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndRecvPacket(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndRecvPacket is a paid mutator transaction binding the contract method 0xd3cffc28.
//
// Solidity: function updateClientAndRecvPacket(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndRecvPacket(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndRecvPacket(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndTimeout is a paid mutator transaction binding the contract method 0xca4c72a0.
//
// Solidity: function updateClientAndTimeout(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpdateClientAndTimeout(opts *bind.TransactOpts, data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "updateClientAndTimeout", data1, data2)
}

// UpdateClientAndTimeout is a paid mutator transaction binding the contract method 0xca4c72a0.
//
// Solidity: function updateClientAndTimeout(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsSession) UpdateClientAndTimeout(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndTimeout(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpdateClientAndTimeout is a paid mutator transaction binding the contract method 0xca4c72a0.
//
// Solidity: function updateClientAndTimeout(bytes data1, bytes data2) payable returns(bool)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpdateClientAndTimeout(data1 []byte, data2 []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpdateClientAndTimeout(&_RelayerFunctions.TransactOpts, data1, data2)
}

// UpgradeClient is a paid mutator transaction binding the contract method 0x8a8e4c5d.
//
// Solidity: function upgradeClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactor) UpgradeClient(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.contract.Transact(opts, "upgradeClient", data)
}

// UpgradeClient is a paid mutator transaction binding the contract method 0x8a8e4c5d.
//
// Solidity: function upgradeClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsSession) UpgradeClient(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpgradeClient(&_RelayerFunctions.TransactOpts, data)
}

// UpgradeClient is a paid mutator transaction binding the contract method 0x8a8e4c5d.
//
// Solidity: function upgradeClient(bytes data) payable returns(bytes)
func (_RelayerFunctions *RelayerFunctionsTransactorSession) UpgradeClient(data []byte) (*types.Transaction, error) {
	return _RelayerFunctions.Contract.UpgradeClient(&_RelayerFunctions.TransactOpts, data)
}
