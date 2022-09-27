package tmdb

import (
	"testing"

	"github.com/crypto-org-chain/cronos/versiondb"
	dbm "github.com/tendermint/tm-db"
)

func TestTMDB(t *testing.T) {
	versiondb.Run(t, func() versiondb.VersionStore {
		return NewStore(dbm.NewMemDB(), dbm.NewMemDB(), dbm.NewMemDB())
	})
}
