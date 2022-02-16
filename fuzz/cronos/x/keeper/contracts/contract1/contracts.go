package contract

import (

	// embed compiled smart contract
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// ByteString is a byte array that serializes to hex
type ByteString []byte

// MarshalJSON serializes ByteArray to hex
func (s ByteString) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(fmt.Sprintf("%x", string(s)))
	return bytes, err
}

// UnmarshalJSON deserializes ByteArray to hex
func (s *ByteString) UnmarshalJSON(data []byte) error {
	var x string
	err := json.Unmarshal(data, &x)
	if err == nil {
		str, e := hex.DecodeString(x)
		*s = str
		err = e
	}

	return err
}

// CompiledContract contains compiled bytecode and abi
type CompiledContract struct {
	ABI abi.ABI
	Bin ByteString
}

const EVMModuleName = "cronos-evm"

var (
	// ModuleCRC20Contract is the compiled cronos erc20 contract
	ModuleCRC20Contract CompiledContract

	// EVMModuleAddress is the native module address for EVM
	EVMModuleAddress common.Address
)

func Fuzz(data []byte) int {
	EVMModuleAddress = common.BytesToAddress(authtypes.NewModuleAddress(EVMModuleName).Bytes())

	err := json.Unmarshal(data, &ModuleCRC20Contract)

	if err != nil {
		return -1
		panic(err)
	}

	if len(ModuleCRC20Contract.Bin) == 0 {
		return -1
		panic("load contract failed")
	}

	return 0
}
