package simulation_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/simulation"
)

// TestParamChanges tests the paramChanges are generated as expected.
func TestParamChanges(t *testing.T) {
	s := rand.NewSource(1)
	r := rand.New(s)

	expected := []struct {
		composedKey string
		key         string
		simValue    string
		subspace    string
	}{
		{"cronos/IbcCroDenom", "IbcCroDenom", "ibc/52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649", "cronos"},
		{"cronos/IbcTimeout", "IbcTimeout", fmt.Sprintf("%v", 6334824724549167320), "cronos"},
		{"cronos/EnableAutoDeployment", "EnableAutoDeployment", fmt.Sprintf("%v", true), "cronos"},
	}

	paramChanges := simulation.ParamChanges(r)

	require.Len(t, paramChanges, 3)

	for i, p := range paramChanges {
		require.Equal(t, expected[i].composedKey, p.ComposedKey())
		require.Equal(t, expected[i].key, p.Key())
		require.Equal(t, expected[i].simValue, p.SimValue()(r))
		require.Equal(t, expected[i].subspace, p.Subspace())
	}
}
