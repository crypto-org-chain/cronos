package precompiles

import (
	_ "embed"
	"errors"

	storetypes "cosmossdk.io/store/types"
	"github.com/crypto-org-chain/cronos/v2/llm"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/llama"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

var (
	llamaABI             abi.ABI
	llamaContractAddress = common.BytesToAddress([]byte{103})

	//go:embed llama/stories15M.bin
	stories []byte

	//go:embed llama/tokenizer.bin
	tokenizer []byte
)

func init() {
	if err := llamaABI.UnmarshalJSON([]byte(llama.ILLamaModuleMetaData.ABI)); err != nil {
		panic(err)
	}
}

type LLamaContract struct {
	BaseContract

	kvGasConfig storetypes.GasConfig
	model       *llm.Model
}

func NewLLamaContract(kvGasConfig storetypes.GasConfig, model *llm.Model) vm.PrecompiledContract {
	return &LLamaContract{
		BaseContract: NewBaseContract(llamaContractAddress),
		kvGasConfig:  kvGasConfig,
		model:        model,
	}
}

func (lc *LLamaContract) Address() common.Address {
	return llamaContractAddress
}

func (lc *LLamaContract) RequiredGas(input []byte) uint64 {
	// base cost to prevent large input size
	return uint64(len(input)) * lc.kvGasConfig.WriteCostPerByte
}

func (lc *LLamaContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	methodID := contract.Input[:4]
	method, err := llamaABI.MethodById(methodID)
	if err != nil {
		return nil, err
	}
	if readonly {
		return nil, errors.New("the method is not readonly")
	}
	args, err := method.Inputs.Unpack(contract.Input[4:])
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}
	prompt := args[0].(string)
	seed := args[1].(int64)
	steps := args[2].(int32)
	res := lc.model.Inference(prompt, 1.0, seed, steps)
	return method.Outputs.Pack(res)
}
