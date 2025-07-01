package simulation

import (
	"fmt"
	"testing"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/types/kv"
)

// TestDecodeStore tests that evm simulation decoder decodes the key value pairs as expected.
func TestDecodeStore(t *testing.T) {
	dec := NewDecodeStore()

	contract := common.HexToAddress("0xabc")
	denom := types.IbcCroDenomDefaultValue

	kvPairs := kv.Pairs{
		Pairs: []kv.Pair{
			{Key: types.KeyPrefixDenomToAutoContract, Value: contract.Bytes()},
			{Key: types.KeyPrefixDenomToExternalContract, Value: contract.Bytes()},
			{Key: types.KeyPrefixContractToDenom, Value: []byte(denom)},
		},
	}

	tests := []struct {
		name        string
		expectedLog string
	}{
		{"ExternalContract", fmt.Sprintf("%v\n%v", contract, contract)},
		{"AutoContract", fmt.Sprintf("%v\n%v", contract, contract)},
		{"Denom", fmt.Sprintf("%v\n%v", denom, denom)},
		{"other", ""},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch i {
			case len(tests) - 1:
				require.Panics(t, func() { dec(kvPairs.Pairs[i], kvPairs.Pairs[i]) }, tt.name)
			default:
				require.Equal(t, tt.expectedLog, dec(kvPairs.Pairs[i], kvPairs.Pairs[i]), tt.name)
			}
		})
	}
}
