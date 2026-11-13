package app

import (
	"math/big"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	legacyparamstypes "github.com/crypto-org-chain/cronos/app/legacy/params/types"
	"github.com/ethereum/go-ethereum/common"
	ethparams "github.com/ethereum/go-ethereum/params"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

const upgradeTestHeight = int64(300)

type UpgradeTestSuite struct {
	suite.Suite

	app *App
	ctx sdk.Context
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) SetupTest() {
	s.app = Setup(s.T(), "")
	s.ctx = s.app.NewContext(false).WithBlockHeight(upgradeTestHeight)
}

func (s *UpgradeTestSuite) applyUpgrade() {
	s.Require().NoError(s.app.UpgradeKeeper.ApplyUpgrade(s.ctx, upgradetypes.Plan{Name: planName, Height: upgradeTestHeight}))
}

func (s *UpgradeTestSuite) deployHistoryContract() {
	for _, p := range evmtypes.DefaultPreinstalls {
		if common.HexToAddress(p.Address) == ethparams.HistoryStorageAddress {
			s.Require().NoError(s.app.EvmKeeper.AddPreinstalls(s.ctx, []evmtypes.Preinstall{p}))
			return
		}
	}
	s.FailNow("EIP-2935 preinstall not found")
}

func testHeaderHash(h int64) common.Hash {
	return common.BigToHash(big.NewInt(h + 1))
}

func (s *UpgradeTestSuite) setHeaderHashes(from, to int64) {
	for h := from; h <= to; h++ {
		s.app.EvmKeeper.SetHeaderHash(s.ctx.WithBlockHeight(h).WithHeaderHash(testHeaderHash(h).Bytes()))
	}
}

func (s *UpgradeTestSuite) historicalInfoStoreHas(h int64) bool {
	store := runtime.KVStoreAdapter(runtime.NewKVStoreService(s.app.keys[stakingtypes.StoreKey]).OpenKVStore(s.ctx))
	return store.Has(stakingtypes.GetHistoricalInfoKey(h))
}

func (s *UpgradeTestSuite) TestHistoricalEntries() {
	headerHashNum := int64(evmtypes.DefaultHeaderHashNum)
	testCases := []struct {
		name        string
		malleate    func()
		expDisabled bool
	}{
		{
			name: "EIP-2935 serves the last header_hash_num blocks",
			malleate: func() {
				s.deployHistoryContract()
				s.setHeaderHashes(upgradeTestHeight-headerHashNum, upgradeTestHeight-1)
			},
			expDisabled: true,
		},
		{
			name: "no history contract",
			malleate: func() {
				s.setHeaderHashes(upgradeTestHeight-headerHashNum, upgradeTestHeight-1)
			},
		},
		{
			name: "history_serve_window below header_hash_num",
			malleate: func() {
				s.deployHistoryContract()
				params := s.app.EvmKeeper.GetParams(s.ctx)
				params.HistoryServeWindow = params.HeaderHashNum - 1
				s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, params))
				s.setHeaderHashes(upgradeTestHeight-headerHashNum, upgradeTestHeight-1)
			},
		},
		{
			name: "contract deployed less than header_hash_num blocks ago",
			malleate: func() {
				s.deployHistoryContract()
				s.setHeaderHashes(upgradeTestHeight-headerHashNum+1, upgradeTestHeight-1)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			before, err := s.app.StakingKeeper.GetParams(s.ctx)
			s.Require().NoError(err)
			s.Require().NotZero(before.HistoricalEntries)

			const firstHistorical = upgradeTestHeight - 20
			for h := firstHistorical; h < upgradeTestHeight; h++ {
				hi := stakingtypes.HistoricalInfo{Header: cmtproto.Header{Height: h}}
				s.Require().NoError(s.app.StakingKeeper.SetHistoricalInfo(s.ctx, h, &hi))
			}
			// an entry that no longer decodes must not stop the pruning
			store := runtime.KVStoreAdapter(runtime.NewKVStoreService(s.app.keys[stakingtypes.StoreKey]).OpenKVStore(s.ctx))
			store.Set(stakingtypes.GetHistoricalInfoKey(upgradeTestHeight-2), []byte{0xff})
			tc.malleate()

			s.applyUpgrade()

			params, err := s.app.StakingKeeper.GetParams(s.ctx)
			s.Require().NoError(err)
			s.Require().Equal(sdk.NewCoin(params.BondDenom, sdkmath.OneInt()), params.KeyRotationFee)
			for h := firstHistorical; h < upgradeTestHeight; h++ {
				s.Require().Equal(!tc.expDisabled, s.historicalInfoStoreHas(h), "height %d", h)
			}
			if !tc.expDisabled {
				s.Require().Equal(before.HistoricalEntries, params.HistoricalEntries)
				return
			}
			s.Require().Zero(params.HistoricalEntries)
			getHash := s.app.EvmKeeper.GetHashFn(s.ctx, uint64(headerHashNum))
			for h := upgradeTestHeight - headerHashNum; h < upgradeTestHeight; h++ {
				s.Require().Equal(testHeaderHash(h), getHash(uint64(h)), "BLOCKHASH(%d)", h)
			}
		})
	}
}

func (s *UpgradeTestSuite) TestKeyRotationFeePool() {
	poolAddr := authtypes.NewModuleAddress(stakingtypes.KeyRotationFeePoolName)
	squatCoins := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(7)))
	testCases := []struct {
		name       string
		malleate   func()
		expBalance sdk.Coins
	}{
		{
			name: "no account at the pool address",
			malleate: func() {
				if acc := s.app.AccountKeeper.GetAccount(s.ctx, poolAddr); acc != nil {
					s.app.AccountKeeper.RemoveAccount(s.ctx, acc)
				}
			},
		},
		{
			name: "plain account pre-funded at the pool address",
			malleate: func() {
				if acc := s.app.AccountKeeper.GetAccount(s.ctx, poolAddr); acc != nil {
					s.app.AccountKeeper.RemoveAccount(s.ctx, acc)
				}
				s.Require().NoError(s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, squatCoins))
				s.Require().NoError(s.app.BankKeeper.SendCoins(s.ctx, authtypes.NewModuleAddress(minttypes.ModuleName), poolAddr, squatCoins))
				_, isModule := s.app.AccountKeeper.GetAccount(s.ctx, poolAddr).(sdk.ModuleAccountI)
				s.Require().False(isModule)
			},
			expBalance: squatCoins,
		},
		{
			name:     "module account already exists",
			malleate: func() { s.app.AccountKeeper.GetModuleAccount(s.ctx, stakingtypes.KeyRotationFeePoolName) },
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.malleate()
			var accNum *uint64
			if acc := s.app.AccountKeeper.GetAccount(s.ctx, poolAddr); acc != nil {
				n := acc.GetAccountNumber()
				accNum = &n
			}

			s.applyUpgrade()

			acc := s.app.AccountKeeper.GetAccount(s.ctx, poolAddr)
			s.Require().NotNil(acc)
			macc, ok := acc.(sdk.ModuleAccountI)
			s.Require().True(ok, "pool account is %T", acc)
			s.Require().Equal(stakingtypes.KeyRotationFeePoolName, macc.GetName())
			if accNum != nil {
				s.Require().Equal(*accNum, acc.GetAccountNumber())
			}
			s.Require().True(tc.expBalance.Equal(s.app.BankKeeper.GetAllBalances(s.ctx, poolAddr)))
		})
	}
}

func (s *UpgradeTestSuite) TestLegacyParamsProposal() {
	content := &legacyparamstypes.ParameterChangeProposal{
		Title:       "title",
		Description: "description",
		Changes:     []legacyparamstypes.ParamChange{{Subspace: "staking", Key: "MaxValidators", Value: `"100"`}},
	}
	msg, err := govv1.NewLegacyContent(content, authtypes.NewModuleAddress(govtypes.ModuleName).String())
	s.Require().NoError(err)
	proposer := sdk.AccAddress("proposer")

	testCases := []struct {
		name   string
		run    func() error
		expErr error
	}{
		{
			name: "proposal stored before the upgrade decodes in queries and genesis export",
			run: func() error {
				proposal, err := govv1.NewProposal([]sdk.Msg{msg}, 1, s.ctx.BlockTime(), s.ctx.BlockTime(), "", "title", "summary", proposer, false)
				s.Require().NoError(err)
				s.Require().NoError(s.app.GovKeeper.SetProposal(s.ctx, proposal))

				res, err := govkeeper.NewQueryServer(&s.app.GovKeeper).Proposal(s.ctx, &govv1.QueryProposalRequest{ProposalId: 1})
				if err != nil {
					return err
				}
				exec, ok := res.Proposal.Messages[0].GetCachedValue().(*govv1.MsgExecLegacyContent)
				s.Require().True(ok)
				s.Require().Equal(content, exec.Content.GetCachedValue())

				genesis, err := govkeeper.ExportGenesis(s.ctx, &s.app.GovKeeper)
				if err != nil {
					return err
				}
				s.Require().Len(genesis.Proposals, 1)
				return nil
			},
		},
		{
			name: "new submission is rejected",
			run: func() error {
				_, err := s.app.GovKeeper.SubmitProposal(s.ctx, []sdk.Msg{msg}, "", "title", "summary", proposer, false)
				return err
			},
			expErr: govtypes.ErrNoProposalHandlerExists,
		},
		{
			name:   "content fails basic validation",
			run:    content.ValidateBasic,
			expErr: legacyparamstypes.ErrRemoved,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			err := tc.run()
			if tc.expErr == nil {
				s.Require().NoError(err)
				return
			}
			s.Require().ErrorIs(err, tc.expErr)
		})
	}
}
