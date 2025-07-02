package simulation

import (
	"bytes"
	"fmt"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/cosmos-sdk/types/kv"
)

// NewDecodeStore returns a decoder function closure that unmarshals the KVPair's
// value to the corresponding cronos type.
func NewDecodeStore() func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		switch {
		case bytes.Equal(kvA.Key[:1], types.KeyPrefixDenomToExternalContract):
			contractA := common.BytesToAddress(kvA.Value).String()
			contractB := common.BytesToAddress(kvB.Value).String()

			return fmt.Sprintf("%v\n%v", contractA, contractB)
		case bytes.Equal(kvA.Key[:1], types.KeyPrefixDenomToAutoContract):
			contractA := common.BytesToAddress(kvA.Value).String()
			contractB := common.BytesToAddress(kvB.Value).String()

			return fmt.Sprintf("%v\n%v", contractA, contractB)
		case bytes.Equal(kvA.Key[:1], types.KeyPrefixContractToDenom):
			denomA := string(kvA.Value)
			denomB := string(kvB.Value)

			return fmt.Sprintf("%v\n%v", denomA, denomB)
		default:
			panic(fmt.Sprintf("invalid evm key prefix %X", kvA.Key[:1]))
		}
	}
}
