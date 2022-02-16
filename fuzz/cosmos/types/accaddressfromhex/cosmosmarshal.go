package cosmostype

import (
	"github.com/cosmos/cosmos-sdk/types"
)

func Fuzz(data []byte) int {
	_, err := types.AccAddressFromHex(string(data))
	if err != nil {
		return 0
	}
	return 1
}
