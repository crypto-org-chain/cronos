package app

import (
	"math/big"
	"testing"
	"time"

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
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protowire"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
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
			// nor a gap, below which nothing else would ever prune
			const gapHeight = upgradeTestHeight - 5
			store.Delete(stakingtypes.GetHistoricalInfoKey(gapHeight))
			tc.malleate()

			s.applyUpgrade()

			params, err := s.app.StakingKeeper.GetParams(s.ctx)
			s.Require().NoError(err)
			s.Require().Equal(sdk.NewCoin(params.BondDenom, sdkmath.OneInt()), params.KeyRotationFee)
			for h := firstHistorical; h < upgradeTestHeight; h++ {
				s.Require().Equal(!tc.expDisabled && h != gapHeight, store.Has(stakingtypes.GetHistoricalInfoKey(h)), "height %d", h)
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

func (s *UpgradeTestSuite) TestStakingQueuePendingSlots() {
	completion := time.Unix(1_900_000_000, 0).UTC()
	testCases := []struct {
		name     string
		indexKey []byte
		malleate func()
		slots    func() ([]time.Time, error)
	}{
		{
			name:     "unbonding delegation queue",
			indexKey: stakingtypes.UBDQueuePendingSlotsKey,
			malleate: func() {
				s.Require().NoError(s.app.StakingKeeper.SetUBDQueueTimeSlice(s.ctx, completion, []stakingtypes.DVPair{{}}))
			},
			slots: func() ([]time.Time, error) { return s.app.StakingKeeper.GetUBDQueuePendingSlots(s.ctx) },
		},
		{
			name:     "redelegation queue",
			indexKey: stakingtypes.RedelegationQueuePendingSlotsKey,
			malleate: func() {
				s.Require().NoError(s.app.StakingKeeper.SetRedelegationQueueTimeSlice(s.ctx, completion, []stakingtypes.DVVTriplet{{}}))
			},
			slots: func() ([]time.Time, error) { return s.app.StakingKeeper.GetRedelegationQueuePendingSlots(s.ctx) },
		},
		{
			name:     "unbonding validator queue",
			indexKey: stakingtypes.ValidatorQueuePendingSlotsKey,
			malleate: func() {
				s.Require().NoError(s.app.StakingKeeper.SetUnbondingValidatorsQueue(s.ctx, completion, upgradeTestHeight, []string{"validator"}))
			},
			slots: func() ([]time.Time, error) {
				slots, err := s.app.StakingKeeper.GetValidatorQueuePendingSlots(s.ctx)
				times := make([]time.Time, 0, len(slots))
				for _, slot := range slots {
					times = append(times, slot.Time)
				}
				return times, err
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.malleate()
			// a queue entry with no index, as on a chain that never ran the v1.8 handler
			s.ctx.KVStore(s.app.keys[stakingtypes.StoreKey]).Delete(tc.indexKey)

			s.applyUpgrade()

			slots, err := tc.slots()
			s.Require().NoError(err)
			s.Require().Len(slots, 1)
			s.Require().True(completion.Equal(slots[0]), "got %s", slots[0])
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
	encodedTrue, err := collections.BoolValue.Encode(true)
	s.Require().NoError(err)
	// The feegrant v1 to v2 migration wrote queue entries with an empty value, which the keeper can't decode.
	feegrantQueueKey := append([]byte{0x01}, "queued-allowance"...)

	testCases := []struct {
		name                  string
		malleate              func()
		expMlDsaCost          uint64
		expFeegrantQueueValue []byte
	}{
		{
			name: "staking, auth and feegrant at their v1.8 versions with v1.8 state",
			malleate: func() {
				vm, err := s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx)
				s.Require().NoError(err)
				vm[stakingtypes.ModuleName], vm[authtypes.ModuleName], vm[feegrant.ModuleName] = 5, 6, 2
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
			expMlDsaCost:          authtypes.DefaultSigVerifyCostMlDsa65,
			expFeegrantQueueValue: encodedTrue,
		},
		{
			name: "modules already at current versions skip their migrations",
			malleate: func() {
				authParams := s.app.AccountKeeper.GetParams(s.ctx)
				authParams.SigVerifyCostMlDsa65 = customMlDsaCost
				s.Require().NoError(s.app.AccountKeeper.Params.Set(s.ctx, authParams))
			},
			expMlDsaCost:          customMlDsaCost,
			expFeegrantQueueValue: []byte{},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			feegrantStore := s.ctx.KVStore(s.app.keys[feegrant.StoreKey])
			feegrantStore.Set(feegrantQueueKey, []byte{})
			tc.malleate()

			s.applyUpgrade()

			vm, err := s.app.UpgradeKeeper.GetModuleVersionMap(s.ctx)
			s.Require().NoError(err)
			s.Require().Equal(uint64(6), vm[stakingtypes.ModuleName])
			s.Require().Equal(uint64(7), vm[authtypes.ModuleName])
			s.Require().Equal(uint64(3), vm[feegrant.ModuleName])
			s.Require().Equal(tc.expFeegrantQueueValue, feegrantStore.Get(feegrantQueueKey))
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
		portID          = "transfer"
		channelID       = "channel-0"
		seq             = uint64(7)
		migratedChannel = "channel-1"
		migratedSeq     = uint64(11)
	)
	v1Key := channelv11.NextSequenceSendV1Key(portID, channelID)
	setV1Seq := func(store storetypes.KVStore) { store.Set(v1Key, sdk.Uint64ToBigEndian(seq)) }
	testCases := []struct {
		name     string
		state    channeltypes.State
		ordering channeltypes.Order
		malleate func(store storetypes.KVStore)
		expAlias bool
	}{
		{
			name:     "open unordered channel, sequence under the v1 key",
			state:    channeltypes.OPEN,
			ordering: channeltypes.UNORDERED,
			malleate: setV1Seq,
			expAlias: true,
		},
		{
			name:     "open unordered channel, sequence already under the v2 key",
			state:    channeltypes.OPEN,
			ordering: channeltypes.UNORDERED,
			malleate: func(storetypes.KVStore) {
				s.app.IBCKeeper.ChannelKeeperV2.SetNextSequenceSend(s.ctx, channelID, seq)
			},
			expAlias: true,
		},
		{
			name:     "open ordered channel",
			state:    channeltypes.OPEN,
			ordering: channeltypes.ORDERED,
			malleate: setV1Seq,
		},
		{
			name:     "closed unordered channel",
			state:    channeltypes.CLOSED,
			ordering: channeltypes.UNORDERED,
			malleate: setV1Seq,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.app.IBCKeeper.ConnectionKeeper.SetConnection(s.ctx, "connection-0", connectiontypes.ConnectionEnd{ClientId: "07-tendermint-0"})
			s.app.IBCKeeper.ChannelKeeper.SetChannel(s.ctx, portID, channelID, channeltypes.NewChannel(
				tc.state, tc.ordering, channeltypes.NewCounterparty(portID, "channel-9"), []string{"connection-0"}, "ics20-1",
			))
			// a second, already-migrated channel iterated after channel-0 catches a copy-back pass that stops early
			s.app.IBCKeeper.ChannelKeeper.SetChannel(s.ctx, portID, migratedChannel, channeltypes.NewChannel(
				channeltypes.OPEN, channeltypes.UNORDERED, channeltypes.NewCounterparty(portID, "channel-8"), []string{"connection-0"}, "ics20-1",
			))
			s.app.IBCKeeper.ChannelKeeperV2.SetNextSequenceSend(s.ctx, migratedChannel, migratedSeq)
			store := s.ctx.KVStore(s.app.keys[ibcexported.StoreKey])
			tc.malleate(store)

			s.applyUpgrade()

			requireSeqs := func(msg string) {
				for id, exp := range map[string]uint64{channelID: seq, migratedChannel: migratedSeq} {
					got, found := s.app.IBCKeeper.ChannelKeeper.GetNextSequenceSend(s.ctx, portID, id)
					s.Require().True(found, msg)
					s.Require().Equal(exp, got, msg)
				}
			}
			requireSeqs("the upgrade must keep the send sequence")
			s.Require().False(store.Has(v1Key))
			aliasedClient, found := s.app.IBCKeeper.ChannelKeeperV2.GetClientForAlias(s.ctx, channelID)
			s.Require().Equal(tc.expAlias, found, "only an OPEN UNORDERED channel gets an IBC v2 alias")
			counterparty, counterpartyFound := s.app.IBCKeeper.ClientV2Keeper.GetClientCounterparty(s.ctx, channelID)
			s.Require().Equal(tc.expAlias, counterpartyFound)
			if tc.expAlias {
				s.Require().Equal("07-tendermint-0", aliasedClient)
				s.Require().Equal("channel-9", counterparty.ClientId)
			}

			s.Require().NoError(s.app.migrateChannelSendSequences(s.ctx))
			requireSeqs("a second run must keep the send sequence")
		})
	}
}

// v1.9 sizes the commit reserve for ML-DSA-65 signatures for every validator, so
// a live chain's block.max_bytes must still fit a full validator set.
func (s *UpgradeTestSuite) TestCommitReserveFitsLiveBlockSize() {
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
		s.Run(tc.name, func() {
			var dataBytes int64
			s.Require().NotPanics(func() {
				dataBytes = cmttypes.MaxDataBytes(tc.maxBytes, tc.evidenceBytes, tc.maxValidators)
			})
			// keep at least half the block for transactions
			s.Require().GreaterOrEqual(dataBytes, tc.maxBytes/2)
		})
	}
}
