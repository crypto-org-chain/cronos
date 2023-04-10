package memiavl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeSetMarshal(t *testing.T) {
	for _, changes := range ChangeSets {
		bz, err := MarshalChangeSet(changes)
		require.NoError(t, err)

		cs, err := UnmarshalChangeSet(bz)
		require.NoError(t, err)
		require.Equal(t, changes, cs)
	}
}
