package isemptyhash

import (
	ethermint "github.com/tharsis/ethermint/types"
)

func Fuzz(data []byte) int {
	//request := new(types.ContractByDenomRequest)
	result := ethermint.IsEmptyHash(string(data))
	if result {
		return 0
	}
	return 1
}
