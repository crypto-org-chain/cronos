package memiavl

import (
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestIterator(t *testing.T) {
	expItems := [][]pair{
		{},
		{{[]byte("hello"), []byte("world")}},
		{
			{[]byte("hello"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
		},
		{
			{[]byte("hello"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
		{
			{[]byte("hello"), []byte("world1")},
			{[]byte("hello00"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
		{
			{[]byte("hello00"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
		{
			{[]byte("aello00"), []byte("world1")},
			{[]byte("aello01"), []byte("world1")},
			{[]byte("aello02"), []byte("world1")},
			{[]byte("aello03"), []byte("world1")},
			{[]byte("aello04"), []byte("world1")},
			{[]byte("aello05"), []byte("world1")},
			{[]byte("aello06"), []byte("world1")},
			{[]byte("aello07"), []byte("world1")},
			{[]byte("aello08"), []byte("world1")},
			{[]byte("aello09"), []byte("world1")},
			{[]byte("aello10"), []byte("world1")},
			{[]byte("aello11"), []byte("world1")},
			{[]byte("aello12"), []byte("world1")},
			{[]byte("aello13"), []byte("world1")},
			{[]byte("aello14"), []byte("world1")},
			{[]byte("aello15"), []byte("world1")},
			{[]byte("aello16"), []byte("world1")},
			{[]byte("aello17"), []byte("world1")},
			{[]byte("aello18"), []byte("world1")},
			{[]byte("aello19"), []byte("world1")},
			{[]byte("aello20"), []byte("world1")},
			{[]byte("hello00"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
		{
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
	}

	tree := NewEmptyTree(0)
	require.Equal(t, expItems[0], collect(tree.Iterator(nil, nil, true)))

	for _, changes := range ChangeSets {
		applyChangeSet(tree, changes)
		_, v, err := tree.SaveVersion(true)
		require.NoError(t, err)
		require.Equal(t, expItems[v], collect(tree.Iterator(nil, nil, true)))
		require.Equal(t, reverse(expItems[v]), collect(tree.Iterator(nil, nil, false)))
	}
}

func TestIteratorRange(t *testing.T) {
	tree := NewEmptyTree(0)
	for _, changes := range ChangeSets[:6] {
		applyChangeSet(tree, changes)
		_, _, err := tree.SaveVersion(true)
		require.NoError(t, err)
	}

	expItems := []pair{
		{[]byte("aello05"), []byte("world1")},
		{[]byte("aello06"), []byte("world1")},
		{[]byte("aello07"), []byte("world1")},
		{[]byte("aello08"), []byte("world1")},
		{[]byte("aello09"), []byte("world1")},
	}
	require.Equal(t, expItems, collect(tree.Iterator([]byte("aello05"), []byte("aello10"), true)))
}

type pair struct {
	key, value []byte
}

func collect(iter dbm.Iterator) []pair {
	result := []pair{}
	for ; iter.Valid(); iter.Next() {
		result = append(result, pair{key: iter.Key(), value: iter.Value()})
	}
	return result
}

func reverse[S ~[]E, E any](s S) S {
	r := make(S, len(s))
	for i, j := 0, len(s)-1; i <= j; i, j = i+1, j-1 {
		r[i], r[j] = s[j], s[i]
	}
	return r
}
