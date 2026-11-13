package app

import (
	"encoding/json"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cryptoenc "github.com/cometbft/cometbft/crypto/encoding"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type ExportTestSuite struct {
	suite.Suite

	app       *App
	blockTime time.Time
	oldPubKey cryptotypes.PubKey
}

func TestExportTestSuite(t *testing.T) {
	suite.Run(t, new(ExportTestSuite))
}

func (s *ExportTestSuite) SetupTest() {
	s.app = Setup(s.T(), "")
	s.blockTime = time.Now().UTC()
	_, err := s.app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1, Time: s.blockTime})
	s.Require().NoError(err)
	_, err = s.app.Commit()
	s.Require().NoError(err)

	vals, err := s.app.StakingKeeper.GetAllValidators(s.uncachedCtx())
	s.Require().NoError(err)
	s.Require().Len(vals, 1)
	s.oldPubKey, err = vals[0].ConsPubKey()
	s.Require().NoError(err)
}

func (s *ExportTestSuite) uncachedCtx() sdk.Context {
	return s.app.NewUncachedContext(false, cmtproto.Header{Height: s.app.LastBlockHeight(), Time: s.blockTime}) //nolint:staticcheck
}

func (s *ExportTestSuite) finalizeAndCommit() {
	_, err := s.app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height:          s.app.LastBlockHeight() + 1,
		Hash:            s.app.LastCommitID().Hash,
		Time:            s.blockTime,
		ProposerAddress: s.oldPubKey.Address(),
	})
	s.Require().NoError(err)
	_, err = s.app.Commit()
	s.Require().NoError(err)
}

func (s *ExportTestSuite) TestExportZeroHeight() {
	testCases := []struct {
		name       string
		malleate   func()
		expPending int
	}{
		{
			name:     "no pending rotation",
			malleate: func() {},
		},
		{
			name: "pending rotation",
			malleate: func() {
				ctx := s.uncachedCtx()
				vals, err := s.app.StakingKeeper.GetAllValidators(ctx)
				s.Require().NoError(err)
				valAddr, err := sdk.ValAddressFromBech32(vals[0].GetOperator())
				s.Require().NoError(err)
				s.Require().NoError(s.app.StakingKeeper.SetConsKeyRotation(ctx, valAddr, s.oldPubKey, ed25519.GenPrivKey().PubKey()))
			},
			expPending: 1,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.malleate()
			// The rotation applies two blocks after it is scheduled, so one more block leaves it pending.
			s.finalizeAndCommit()
			pending, err := s.app.StakingKeeper.PendingConsKeyRotations(s.uncachedCtx())
			s.Require().NoError(err)
			s.Require().Len(pending, tc.expPending)

			exported, err := s.app.ExportAppStateAndValidators(true, nil, nil)
			s.Require().NoError(err)
			var appState GenesisState
			s.Require().NoError(json.Unmarshal(exported.AppState, &appState))
			var stakingGenesis stakingtypes.GenesisState
			s.app.AppCodec().MustUnmarshalJSON(appState[stakingtypes.ModuleName], &stakingGenesis)
			s.Require().Len(stakingGenesis.PendingConsensusKeyRotations, tc.expPending)
			for _, r := range stakingGenesis.PendingConsensusKeyRotations {
				s.Require().Equal(1+stakingtypes.ConsensusUpdateDelay, r.ApplyHeight)
			}

			validators := make([]abci.ValidatorUpdate, len(exported.Validators))
			for i, val := range exported.Validators {
				pubKey, err := cryptoenc.PubKeyToProto(val.PubKey)
				s.Require().NoError(err)
				validators[i] = abci.ValidatorUpdate{PubKey: pubKey, Power: val.Power}
			}
			// InitChain rejects a genesis whose validators differ from the ones staking returns.
			restarted, _ := setup(false, 5)
			_, err = restarted.InitChain(&abci.RequestInitChain{
				ChainId:         TestAppChainID,
				Time:            s.blockTime,
				Validators:      validators,
				ConsensusParams: DefaultConsensusParams,
				AppStateBytes:   exported.AppState,
				InitialHeight:   1,
			})
			s.Require().NoError(err)
		})
	}
}
