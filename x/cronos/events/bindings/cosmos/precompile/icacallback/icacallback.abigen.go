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
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"seq\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"ack\",\"type\":\"bytes\"}],\"name\":\"OnPacketResult\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"acknowledgement\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"seq\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"ack\",\"type\":\"bytes\"}],\"name\":\"onPacketResultCallback\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561000f575f80fd5b506107a98061001d5f395ff3fe608060405260043610610028575f3560e01c8063968bcba31461002c578063a6d9a1c214610068575b5f80fd5b348015610037575f80fd5b50610052600480360381019061004d919061022d565b610098565b60405161005f91906102e2565b60405180910390f35b610082600480360381019061007d9190610363565b610133565b60405161008f91906103da565b60405180910390f35b6002602052805f5260405f205f9150905080546100b490610420565b80601f01602080910402602001604051908101604052809291908181526020018280546100e090610420565b801561012b5780601f106101025761010080835404028352916020019161012b565b820191905f5260205f20905b81548152906001019060200180831161010e57829003601f168201915b505050505081565b5f835f806101000a81548167ffffffffffffffff021916908367ffffffffffffffff16021790555082826001918261016c92919061062d565b50828260025f8767ffffffffffffffff1667ffffffffffffffff1681526020019081526020015f2091826101a192919061062d565b507f8e0c6cb5698eba8240951fde76f9e06a0844d4285c0e56f4cedf1415d03703fc8484846040516101d593929190610743565b60405180910390a1600190509392505050565b5f80fd5b5f80fd5b5f67ffffffffffffffff82169050919050565b61020c816101f0565b8114610216575f80fd5b50565b5f8135905061022781610203565b92915050565b5f60208284031215610242576102416101e8565b5b5f61024f84828501610219565b91505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f5b8381101561028f578082015181840152602081019050610274565b5f8484015250505050565b5f601f19601f8301169050919050565b5f6102b482610258565b6102be8185610262565b93506102ce818560208601610272565b6102d78161029a565b840191505092915050565b5f6020820190508181035f8301526102fa81846102aa565b905092915050565b5f80fd5b5f80fd5b5f80fd5b5f8083601f84011261032357610322610302565b5b8235905067ffffffffffffffff8111156103405761033f610306565b5b60208301915083600182028301111561035c5761035b61030a565b5b9250929050565b5f805f6040848603121561037a576103796101e8565b5b5f61038786828701610219565b935050602084013567ffffffffffffffff8111156103a8576103a76101ec565b5b6103b48682870161030e565b92509250509250925092565b5f8115159050919050565b6103d4816103c0565b82525050565b5f6020820190506103ed5f8301846103cb565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52602260045260245ffd5b5f600282049050600182168061043757607f821691505b60208210810361044a576104496103f3565b5b50919050565b5f82905092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b5f819050815f5260205f209050919050565b5f6020601f8301049050919050565b5f82821b905092915050565b5f600883026104e37fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff826104a8565b6104ed86836104a8565b95508019841693508086168417925050509392505050565b5f819050919050565b5f819050919050565b5f61053161052c61052784610505565b61050e565b610505565b9050919050565b5f819050919050565b61054a83610517565b61055e61055682610538565b8484546104b4565b825550505050565b5f90565b610572610566565b61057d818484610541565b505050565b5b818110156105a0576105955f8261056a565b600181019050610583565b5050565b601f8211156105e5576105b681610487565b6105bf84610499565b810160208510156105ce578190505b6105e26105da85610499565b830182610582565b50505b505050565b5f82821c905092915050565b5f6106055f19846008026105ea565b1980831691505092915050565b5f61061d83836105f6565b9150826002028217905092915050565b6106378383610450565b67ffffffffffffffff8111156106505761064f61045a565b5b61065a8254610420565b6106658282856105a4565b5f601f831160018114610692575f8415610680578287013590505b61068a8582610612565b8655506106f1565b601f1984166106a086610487565b5f5b828110156106c7578489013582556001820191506020850194506020810190506106a2565b868310156106e457848901356106e0601f8916826105f6565b8355505b6001600288020188555050505b50505050505050565b610703816101f0565b82525050565b828183375f83830152505050565b5f6107228385610262565b935061072f838584610709565b6107388361029a565b840190509392505050565b5f6040820190506107565f8301866106fa565b8181036020830152610769818486610717565b905094935050505056fea2646970667358221220209d1395f552db65e120063a95265a8c5d0c258b4fabfa7545d202c4e45287bb64736f6c63430008150033",
}

// ICACallbackABI is the input ABI used to generate the binding from.
// Deprecated: Use ICACallbackMetaData.ABI instead.
var ICACallbackABI = ICACallbackMetaData.ABI

// ICACallbackBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use ICACallbackMetaData.Bin instead.
var ICACallbackBin = ICACallbackMetaData.Bin

// DeployICACallback deploys a new Ethereum contract, binding an instance of ICACallback to it.
func DeployICACallback(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *ICACallback, error) {
	parsed, err := ICACallbackMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(ICACallbackBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &ICACallback{ICACallbackCaller: ICACallbackCaller{contract: contract}, ICACallbackTransactor: ICACallbackTransactor{contract: contract}, ICACallbackFilterer: ICACallbackFilterer{contract: contract}}, nil
}

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

// Acknowledgement is a free data retrieval call binding the contract method 0x968bcba3.
//
// Solidity: function acknowledgement(uint64 ) view returns(bytes)
func (_ICACallback *ICACallbackCaller) Acknowledgement(opts *bind.CallOpts, arg0 uint64) ([]byte, error) {
	var out []interface{}
	err := _ICACallback.contract.Call(opts, &out, "acknowledgement", arg0)

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// Acknowledgement is a free data retrieval call binding the contract method 0x968bcba3.
//
// Solidity: function acknowledgement(uint64 ) view returns(bytes)
func (_ICACallback *ICACallbackSession) Acknowledgement(arg0 uint64) ([]byte, error) {
	return _ICACallback.Contract.Acknowledgement(&_ICACallback.CallOpts, arg0)
}

// Acknowledgement is a free data retrieval call binding the contract method 0x968bcba3.
//
// Solidity: function acknowledgement(uint64 ) view returns(bytes)
func (_ICACallback *ICACallbackCallerSession) Acknowledgement(arg0 uint64) ([]byte, error) {
	return _ICACallback.Contract.Acknowledgement(&_ICACallback.CallOpts, arg0)
}

// OnPacketResultCallback is a paid mutator transaction binding the contract method 0xa6d9a1c2.
//
// Solidity: function onPacketResultCallback(uint64 seq, bytes ack) payable returns(bool)
func (_ICACallback *ICACallbackTransactor) OnPacketResultCallback(opts *bind.TransactOpts, seq uint64, ack []byte) (*types.Transaction, error) {
	return _ICACallback.contract.Transact(opts, "onPacketResultCallback", seq, ack)
}

// OnPacketResultCallback is a paid mutator transaction binding the contract method 0xa6d9a1c2.
//
// Solidity: function onPacketResultCallback(uint64 seq, bytes ack) payable returns(bool)
func (_ICACallback *ICACallbackSession) OnPacketResultCallback(seq uint64, ack []byte) (*types.Transaction, error) {
	return _ICACallback.Contract.OnPacketResultCallback(&_ICACallback.TransactOpts, seq, ack)
}

// OnPacketResultCallback is a paid mutator transaction binding the contract method 0xa6d9a1c2.
//
// Solidity: function onPacketResultCallback(uint64 seq, bytes ack) payable returns(bool)
func (_ICACallback *ICACallbackTransactorSession) OnPacketResultCallback(seq uint64, ack []byte) (*types.Transaction, error) {
	return _ICACallback.Contract.OnPacketResultCallback(&_ICACallback.TransactOpts, seq, ack)
}

// ICACallbackOnPacketResultIterator is returned from FilterOnPacketResult and is used to iterate over the raw logs and unpacked data for OnPacketResult events raised by the ICACallback contract.
type ICACallbackOnPacketResultIterator struct {
	Event *ICACallbackOnPacketResult // Event containing the contract specifics and raw log

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
func (it *ICACallbackOnPacketResultIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ICACallbackOnPacketResult)
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
		it.Event = new(ICACallbackOnPacketResult)
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
func (it *ICACallbackOnPacketResultIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ICACallbackOnPacketResultIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ICACallbackOnPacketResult represents a OnPacketResult event raised by the ICACallback contract.
type ICACallbackOnPacketResult struct {
	Seq uint64
	Ack []byte
	Raw types.Log // Blockchain specific contextual infos
}

// FilterOnPacketResult is a free log retrieval operation binding the contract event 0x8e0c6cb5698eba8240951fde76f9e06a0844d4285c0e56f4cedf1415d03703fc.
//
// Solidity: event OnPacketResult(uint64 seq, bytes ack)
func (_ICACallback *ICACallbackFilterer) FilterOnPacketResult(opts *bind.FilterOpts) (*ICACallbackOnPacketResultIterator, error) {

	logs, sub, err := _ICACallback.contract.FilterLogs(opts, "OnPacketResult")
	if err != nil {
		return nil, err
	}
	return &ICACallbackOnPacketResultIterator{contract: _ICACallback.contract, event: "OnPacketResult", logs: logs, sub: sub}, nil
}

// WatchOnPacketResult is a free log subscription operation binding the contract event 0x8e0c6cb5698eba8240951fde76f9e06a0844d4285c0e56f4cedf1415d03703fc.
//
// Solidity: event OnPacketResult(uint64 seq, bytes ack)
func (_ICACallback *ICACallbackFilterer) WatchOnPacketResult(opts *bind.WatchOpts, sink chan<- *ICACallbackOnPacketResult) (event.Subscription, error) {

	logs, sub, err := _ICACallback.contract.WatchLogs(opts, "OnPacketResult")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ICACallbackOnPacketResult)
				if err := _ICACallback.contract.UnpackLog(event, "OnPacketResult", log); err != nil {
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

// ParseOnPacketResult is a log parse operation binding the contract event 0x8e0c6cb5698eba8240951fde76f9e06a0844d4285c0e56f4cedf1415d03703fc.
//
// Solidity: event OnPacketResult(uint64 seq, bytes ack)
func (_ICACallback *ICACallbackFilterer) ParseOnPacketResult(log types.Log) (*ICACallbackOnPacketResult, error) {
	event := new(ICACallbackOnPacketResult)
	if err := _ICACallback.contract.UnpackLog(event, "OnPacketResult", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
