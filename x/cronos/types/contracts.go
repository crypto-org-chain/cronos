package types

import (
	// embed compiled smart contract
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
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
	//go:embed contracts/ModuleCRC20.json
	cronosCRC20JSON []byte

	// ModuleCRC20Contract is the compiled cronos crc20 contract
	ModuleCRC20Contract CompiledContract

	//go:embed contracts/ModuleCRC21.json
	cronosCRC21JSON []byte

	// ModuleCRC21Contract is the compiled cronos crc21 contract
	ModuleCRC21Contract CompiledContract

	// EVMModuleAddress is the native module address for EVM
	EVMModuleAddress common.Address
)

func init() {
	EVMModuleAddress = common.BytesToAddress(authtypes.NewModuleAddress(EVMModuleName).Bytes())

	err := json.Unmarshal(cronosCRC20JSON, &ModuleCRC20Contract)
	if err != nil {
		panic(err)
	}

	if len(ModuleCRC20Contract.Bin) == 0 {
		panic("load contract failed")
	}

	err = json.Unmarshal(cronosCRC21JSON, &ModuleCRC21Contract)
	if err != nil {
		panic(err)
	}

	if len(ModuleCRC21Contract.Bin) == 0 {
		panic("load contract failed")
	}
}
