package rootmulti

import (
	"testing"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/store/types"
	"github.com/stretchr/testify/require"
)

func TestLastCommitID(t *testing.T) {
	store := NewStore(t.TempDir(), log.NewNopLogger(), true)
	require.Equal(t, types.CommitID{}, store.LastCommitID())
}
