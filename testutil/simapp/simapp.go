package simapp

import (
	"time"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	tmdb "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

	"github.com/crypto-org-chain/cronos/v2/app"
)

// New creates application instance with in-memory database and disabled logging.
func New(dir string) *app.App {
	db := tmdb.NewMemDB()
	logger := log.NewNopLogger()

	a := app.New(logger, db, nil, true, simtestutil.EmptyAppOptions{})
	// InitChain updates deliverState which is required when app.NewContext is called
	a.InitChain(&abci.RequestInitChain{
		ConsensusParams: defaultConsensusParams,
		AppStateBytes:   []byte("{}"),
	})
	return a
}

var defaultConsensusParams = &tmproto.ConsensusParams{
	Block: &tmproto.BlockParams{
		MaxBytes: 200000,
		MaxGas:   2000000,
	},
	Evidence: &tmproto.EvidenceParams{
		MaxAgeNumBlocks: 302400,
		MaxAgeDuration:  504 * time.Hour, // 3 weeks is the max duration
		MaxBytes:        10000,
	},
	Validator: &tmproto.ValidatorParams{
		PubKeyTypes: []string{
			tmtypes.ABCIPubKeyTypeEd25519,
		},
	},
}
