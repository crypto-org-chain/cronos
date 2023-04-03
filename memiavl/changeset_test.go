package memiavl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func ChangeSetMarshal(t *testing.T) {
	for _, changes := range ChangeSets {
		bz, err := changes.Marshal()
		require.NoError(t, err)

		var cs ChangeSet
		require.NoError(t, cs.Unmarshal(bz))
		require.Equal(t, changes, cs)
	}
}
