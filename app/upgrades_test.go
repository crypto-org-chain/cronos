package app

import (
	"math/big"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	connectiontypes "github.com/cosmos/ibc-go/v11/modules/core/03-connection/types"
	channelv11 "github.com/cosmos/ibc-go/v11/modules/core/04-channel/migrations/v11"
	channeltypes "github.com/cosmos/ibc-go/v11/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v11/modules/core/exported"
	"github.com/ethereum/go-ethereum/common"
	ethparams "github.com/ethereum/go-ethereum/params"
	legacyparamstypes "github.com/evmos/ethermint/types/legacyparams"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	feemarkettypes "github.com/evmos/ethermint/x/feemarket/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protowire"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
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
		{
			name: "only legacy header hashes behind a deployed contract",
			malleate: func() {
				s.setHeaderHashes(upgradeTestHeight-headerHashNum, upgradeTestHeight-1)
				s.deployHistoryContract()
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
			store := s.ctx.KVStore(s.app.keys[stakingtypes.StoreKey])
			store.Set(stakingtypes.GetHistoricalInfoKey(upgradeTestHeight-2), []byte{0xff})
			tc.malleate()

			s.applyUpgrade()

			params, err := s.app.StakingKeeper.GetParams(s.ctx)
			s.Require().NoError(err)
			s.Require().Equal(sdk.NewCoin(params.BondDenom, sdkmath.OneInt()), params.KeyRotationFee)
			for h := firstHistorical; h < upgradeTestHeight; h++ {
				s.Require().Equal(!tc.expDisabled, store.Has(stakingtypes.GetHistoricalInfoKey(h)), "height %d", h)
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
			name:     "no account at the pool address",
			malleate: func() {},
		},
		{
			name: "plain account pre-funded at the pool address",
			malleate: func() {
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
			if acc := s.app.AccountKeeper.GetAccount(s.ctx, poolAddr); acc != nil {
				s.app.AccountKeeper.RemoveAccount(s.ctx, acc)
			}
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

func (s *UpgradeTestSuite) TestModuleMigrations() {
	const customMlDsaCost = uint64(123)
	testCases := []struct {
		name         string
		malleate     func()
		expMlDsaCost uint64
	}{
		{
			name: "staking and auth at their v1.8 versions with v1.8 params",
			malleate: func() {
				vm, err := s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx)
				s.Require().NoError(err)
				vm[stakingtypes.ModuleName], vm[authtypes.ModuleName] = 5, 6
				s.Require().NoError(s.app.UpgradeKeeper.SetModuleVersionMap(s.ctx, vm))

				authParams := s.app.AccountKeeper.GetParams(s.ctx)
				authParams.SigVerifyCostMlDsa65 = 0
				s.Require().NoError(s.app.AccountKeeper.Params.Set(s.ctx, authParams))

				// v1.8 params have no key_rotation_fee field, which decodes differently from an encoded empty coin.
				// The handler then overwrites the fee, so this only shows staking's migration accepts v1.8 params.
				stakingParams, err := s.app.StakingKeeper.GetParams(s.ctx)
				s.Require().NoError(err)
				bz, err := s.app.appCodec.Marshal(&stakingParams)
				s.Require().NoError(err)
				s.ctx.KVStore(s.app.keys[stakingtypes.StoreKey]).Set(stakingtypes.ParamsKey, s.dropProtoField(bz, 7))
				stakingParams, err = s.app.StakingKeeper.GetParams(s.ctx)
				s.Require().NoError(err)
				s.Require().True(stakingParams.KeyRotationFee.Amount.IsNil())
			},
			expMlDsaCost: authtypes.DefaultSigVerifyCostMlDsa65,
		},
		{
			name: "modules already at current versions skip their migrations",
			malleate: func() {
				authParams := s.app.AccountKeeper.GetParams(s.ctx)
				authParams.SigVerifyCostMlDsa65 = customMlDsaCost
				s.Require().NoError(s.app.AccountKeeper.Params.Set(s.ctx, authParams))
			},
			expMlDsaCost: customMlDsaCost,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.malleate()

			s.applyUpgrade()

			vm, err := s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx)
			s.Require().NoError(err)
			s.Require().Equal(uint64(6), vm[stakingtypes.ModuleName])
			s.Require().Equal(uint64(7), vm[authtypes.ModuleName])
			s.Require().Equal(tc.expMlDsaCost, s.app.AccountKeeper.GetParams(s.ctx).SigVerifyCostMlDsa65)
			stakingParams, err := s.app.StakingKeeper.GetParams(s.ctx)
			s.Require().NoError(err)
			s.Require().Equal(sdk.NewCoin(stakingParams.BondDenom, sdkmath.OneInt()), stakingParams.KeyRotationFee)
		})
	}
}

func (s *UpgradeTestSuite) dropProtoField(bz []byte, field protowire.Number) []byte {
	var out []byte
	for len(bz) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(bz)
		s.Require().GreaterOrEqual(tagLen, 0)
		valLen := protowire.ConsumeFieldValue(num, typ, bz[tagLen:])
		s.Require().GreaterOrEqual(valLen, 0)
		if num != field {
			out = append(out, bz[:tagLen+valLen]...)
		}
		bz = bz[tagLen+valLen:]
	}
	return out
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

func (s *UpgradeTestSuite) TestHistoricalParamsFromLegacyStore() {
	amino := codec.NewLegacyAmino()
	baseFee := sdkmath.NewInt(5000)

	testCases := []struct {
		name     string
		malleate func(params storetypes.KVStore)
		check    func()
	}{
		{
			name: "evm reads its x/params subspace",
			malleate: func(params storetypes.KVStore) {
				s.ctx.KVStore(s.app.GetKey(evmtypes.StoreKey)).Delete(evmtypes.KeyPrefixParams)
				params.Set([]byte("evm/EVMDenom"), amino.MustMarshalJSON("basetcro"))
			},
			check: func() { s.Require().Equal("basetcro", s.app.EvmKeeper.GetParams(s.ctx).EvmDenom) },
		},
		{
			name: "feemarket reads its x/params subspace",
			malleate: func(params storetypes.KVStore) {
				s.ctx.KVStore(s.app.GetKey(feemarkettypes.StoreKey)).Delete(feemarkettypes.ParamsKey)
				params.Set([]byte("feemarket/BaseFee"), amino.MustMarshalJSON(baseFee))
			},
			check: func() { s.Require().Equal(baseFee, s.app.FeeMarketKeeper.GetParams(s.ctx).BaseFee) },
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.malleate(s.ctx.KVStore(s.app.GetKey(paramsStoreKey)))
			tc.check()
		})
	}
}

func (s *UpgradeTestSuite) TestIBCSendSequence() {
	const (
		portID    = "transfer"
		channelID = "channel-0"
		seq       = uint64(7)
	)
	v1Key := channelv11.NextSequenceSendV1Key(portID, channelID)
	testCases := []struct {
		name     string
		malleate func(store storetypes.KVStore)
	}{
		{
			name:     "sequence under the v1 key",
			malleate: func(store storetypes.KVStore) { store.Set(v1Key, sdk.Uint64ToBigEndian(seq)) },
		},
		{
			name: "sequence already under the v2 key",
			malleate: func(storetypes.KVStore) {
				s.app.IBCKeeper.ChannelKeeperV2.SetNextSequenceSend(s.ctx, channelID, seq)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.app.IBCKeeper.ConnectionKeeper.SetConnection(s.ctx, "connection-0", connectiontypes.ConnectionEnd{ClientId: "07-tendermint-0"})
			s.app.IBCKeeper.ChannelKeeper.SetChannel(s.ctx, portID, channelID, channeltypes.NewChannel(
				channeltypes.OPEN, channeltypes.UNORDERED, channeltypes.NewCounterparty(portID, "channel-9"), []string{"connection-0"}, "ics20-1",
			))
			store := s.ctx.KVStore(s.app.keys[ibcexported.StoreKey])
			tc.malleate(store)

			s.applyUpgrade()

			got, found := s.app.IBCKeeper.ChannelKeeper.GetNextSequenceSend(s.ctx, portID, channelID)
			s.Require().True(found)
			s.Require().Equal(seq, got)
			s.Require().False(store.Has(v1Key))
			aliasedClient, found := s.app.IBCKeeper.ChannelKeeperV2.GetClientForAlias(s.ctx, channelID)
			s.Require().True(found, "an OPEN UNORDERED channel must get an IBC v2 alias")
			s.Require().Equal("07-tendermint-0", aliasedClient)
			counterparty, found := s.app.IBCKeeper.ClientV2Keeper.GetClientCounterparty(s.ctx, channelID)
			s.Require().True(found)
			s.Require().Equal("channel-9", counterparty.ClientId)

			s.Require().NoError(s.app.migrateChannelSendSequences(s.ctx))
			got, _ = s.app.IBCKeeper.ChannelKeeper.GetNextSequenceSend(s.ctx, portID, channelID)
			s.Require().Equal(seq, got, "a second run must keep the send sequence")
		})
	}
}

// v1.9 sizes the commit reserve for ML-DSA-65 signatures for every validator, so
// a live chain's block.max_bytes must still fit a full validator set.
func TestCommitReserveFitsLiveBlockSize(t *testing.T) {
	testCases := []struct {
		name          string
		maxBytes      int64
		evidenceBytes int64
		maxValidators int
	}{
		// values from consensus and staking params as of 2026-11-14
		{name: "mainnet", maxBytes: 1048576, evidenceBytes: 150000, maxValidators: 30},
		{name: "testnet", maxBytes: 1048576, evidenceBytes: 150000, maxValidators: 35},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var dataBytes int64
			require.NotPanics(t, func() {
				dataBytes = cmttypes.MaxDataBytes(tc.maxBytes, tc.evidenceBytes, tc.maxValidators)
			})
			// keep at least half the block for transactions
			require.GreaterOrEqual(t, dataBytes, tc.maxBytes/2)
		})
	}
}
