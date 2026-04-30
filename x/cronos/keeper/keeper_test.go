package keeper_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/cometbft/cometbft/crypto/tmhash"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmversion "github.com/cometbft/cometbft/proto/tendermint/version"
	"github.com/cometbft/cometbft/version"
	"github.com/crypto-org-chain/cronos/app"
	cronosmodulekeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	keepertest "github.com/crypto-org-chain/cronos/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	ethermint "github.com/evmos/ethermint/types"
	"github.com/evmos/ethermint/x/evm/statedb"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	denom        = "testdenom"
	denomGravity = "gravity0x0000000000000000000000000000000000000000"
)

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

type KeeperTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *app.App

	// EVM helpers
	evmParam evmtypes.Params

	address common.Address
}

func (suite *KeeperTestSuite) DoSetupTest(t *testing.T) {
	t.Helper()
	// account key
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	suite.address = common.BytesToAddress(priv.PubKey().Address().Bytes())

	// consensus key
	priv, err = ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	consAddress := sdk.ConsAddress(priv.PubKey().Address())

	suite.app = app.Setup(t, sdk.AccAddress(suite.address.Bytes()).String())
	blockIDHash := tmhash.Sum([]byte("block_id"))
	hash := tmhash.Sum([]byte("partset_header"))
	suite.ctx = suite.app.NewContext(false).WithBlockHeader(tmproto.Header{
		Height:          1,
		ChainID:         app.TestAppChainID,
		Time:            time.Now().UTC(),
		ProposerAddress: consAddress.Bytes(),
		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
		LastBlockId: tmproto.BlockID{
			Hash: blockIDHash,
			PartSetHeader: tmproto.PartSetHeader{
				Total: 11,
				Hash:  hash,
			},
		},
		AppHash:            tmhash.Sum([]byte("app")),
		DataHash:           tmhash.Sum([]byte("data")),
		EvidenceHash:       tmhash.Sum([]byte("evidence")),
		ValidatorsHash:     tmhash.Sum([]byte("validators")),
		NextValidatorsHash: tmhash.Sum([]byte("next_validators")),
		ConsensusHash:      tmhash.Sum([]byte("consensus")),
		LastResultsHash:    tmhash.Sum([]byte("last_result")),
	})

	// initialize the validator
	acc := &ethermint.EthAccount{
		BaseAccount: authtypes.NewBaseAccount(sdk.AccAddress(suite.address.Bytes()), nil, 0, 0),
		CodeHash:    common.BytesToHash(ethcrypto.Keccak256(nil)).String(),
	}

	acc.AccountNumber = suite.app.AccountKeeper.NextAccountNumber(suite.ctx)
	suite.app.AccountKeeper.SetAccount(suite.ctx, acc)

	valAddr := sdk.ValAddress(suite.address.Bytes())
	validator, err := stakingtypes.NewValidator(valAddr.String(), priv.PubKey(), stakingtypes.Description{})
	require.NoError(t, err)
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	err = suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
	require.NoError(t, err)

	suite.evmParam = suite.app.EvmKeeper.GetParams(suite.ctx)
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.DoSetupTest(suite.T())
}

func (suite *KeeperTestSuite) MintCoins(address sdk.AccAddress, coins sdk.Coins) error {
	err := suite.app.BankKeeper.MintCoins(suite.ctx, minttypes.ModuleName, coins)
	if err != nil {
		return err
	}
	err = suite.app.BankKeeper.SendCoinsFromModuleToAccount(suite.ctx, minttypes.ModuleName, address, coins)
	if err != nil {
		return err
	}
	return nil
}

func (suite *KeeperTestSuite) RegisterSourceToken(
	contractAddress, symbol string, decimal uint32,
) error {
	denom := "cronos" + contractAddress
	msg := types.MsgUpdateTokenMapping{
		Denom:    denom,
		Contract: contractAddress,
		Symbol:   symbol,
		Decimal:  decimal,
	}
	return suite.app.CronosKeeper.RegisterOrUpdateTokenMapping(suite.ctx, &msg)
}

func (suite *KeeperTestSuite) TestDenomContractMap() {
	denom1 := denom + "1"
	denom2 := denom + "2"

	autoContract := common.BigToAddress(big.NewInt(1))
	externalContract := common.BigToAddress(big.NewInt(2))

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			"success, happy path",
			func() {
				keeper := suite.app.CronosKeeper

				_, found := keeper.GetContractByDenom(suite.ctx, denom1)
				suite.Require().False(found)

				err := keeper.SetExternalContractForDenom(suite.ctx, denom1, externalContract)
				suite.Require().NoError(err)

				contract, found := keeper.GetContractByDenom(suite.ctx, denom1)
				suite.Require().True(found)
				suite.Require().Equal(externalContract, contract)
				denomByContract, found := keeper.GetDenomByContract(suite.ctx, externalContract)
				suite.Require().True(found)
				suite.Require().Equal(denom1, denomByContract)
			},
		},
		{
			"success, delete external falls back to legacy auto mapping",
			func() {
				keeper := suite.app.CronosKeeper
				store := suite.ctx.KVStore(suite.app.GetKey(types.StoreKey))

				legacyDenom := denom + "legacy"
				legacyAuto := common.BigToAddress(big.NewInt(3))
				legacyExternal := common.BigToAddress(big.NewInt(4))

				// Simulate legacy state where both mappings exist.
				store.Set(types.DenomToAutoContractKey(legacyDenom), legacyAuto.Bytes())
				store.Set(types.ContractToDenomKey(legacyAuto.Bytes()), []byte(legacyDenom))
				store.Set(types.DenomToExternalContractKey(legacyDenom), legacyExternal.Bytes())
				store.Set(types.ContractToDenomKey(legacyExternal.Bytes()), []byte(legacyDenom))

				_, found := keeper.GetDenomByContract(suite.ctx, legacyAuto)
				suite.Require().False(found)

				deleted := keeper.DeleteExternalContractForDenom(suite.ctx, legacyDenom)
				suite.Require().True(deleted)

				contract, found := keeper.GetContractByDenom(suite.ctx, legacyDenom)
				suite.Require().True(found)
				suite.Require().Equal(legacyAuto, contract)
				denomByAuto, found := keeper.GetDenomByContract(suite.ctx, legacyAuto)
				suite.Require().True(found)
				suite.Require().Equal(legacyDenom, denomByAuto)
				_, found = keeper.GetDenomByContract(suite.ctx, legacyExternal)
				suite.Require().False(found)
			},
		},
		{
			"success, cross-check rejects stale reverse mapping with external active",
			func() {
				keeper := suite.app.CronosKeeper
				store := suite.ctx.KVStore(suite.app.GetKey(types.StoreKey))

				legacyDenom := denom + "crosscheck"
				legacyAuto := common.BigToAddress(big.NewInt(5))
				legacyExternal := common.BigToAddress(big.NewInt(6))

				// External mapping is active for denom.
				store.Set(types.DenomToExternalContractKey(legacyDenom), legacyExternal.Bytes())
				store.Set(types.ContractToDenomKey(legacyExternal.Bytes()), []byte(legacyDenom))
				// Stale reverse entry for auto contract remains.
				store.Set(types.ContractToDenomKey(legacyAuto.Bytes()), []byte(legacyDenom))

				_, found := keeper.GetDenomByContract(suite.ctx, legacyAuto)
				suite.Require().False(found)
				denomByExternal, found := keeper.GetDenomByContract(suite.ctx, legacyExternal)
				suite.Require().True(found)
				suite.Require().Equal(legacyDenom, denomByExternal)
			},
		},
		{
			"success, source denom external matches embedded address",
			func() {
				keeper := suite.app.CronosKeeper

				sourceAuto := common.BigToAddress(big.NewInt(12))
				sourceDenom := "cronos" + sourceAuto.Hex()

				err := keeper.SetExternalContractForDenom(suite.ctx, sourceDenom, sourceAuto)
				suite.Require().NoError(err)

				contract, found := keeper.GetContractByDenom(suite.ctx, sourceDenom)
				suite.Require().True(found)
				suite.Require().Equal(sourceAuto, contract)

				deleted := keeper.DeleteExternalContractForDenom(suite.ctx, sourceDenom)
				suite.Require().True(deleted)

				_, found = keeper.GetContractByDenom(suite.ctx, sourceDenom)
				suite.Require().False(found)
			},
		},
		{
			"success, SetExternalContractForDenom accepts mismatched contract for source denom",
			func() {
				keeper := suite.app.CronosKeeper

				sourceAuto := common.BigToAddress(big.NewInt(12))
				sourceDenom := "cronos" + sourceAuto.Hex()
				mismatched := common.BigToAddress(big.NewInt(13))

				err := keeper.SetExternalContractForDenom(suite.ctx, sourceDenom, mismatched)
				suite.Require().NoError(err)
				contract, found := keeper.GetContractByDenom(suite.ctx, sourceDenom)
				suite.Require().True(found)
				suite.Require().Equal(mismatched, contract)
			},
		},
		{
			"failure, SetAutoContractForDenom rejects mismatched contract for source denom",
			func() {
				keeper := suite.app.CronosKeeper

				sourceAuto := common.BigToAddress(big.NewInt(14))
				sourceDenom := "cronos" + sourceAuto.Hex()
				mismatched := common.BigToAddress(big.NewInt(15))

				err := keeper.SetAutoContractForDenom(suite.ctx, sourceDenom, mismatched)
				suite.Require().Error(err)
				_, found := keeper.GetContractByDenom(suite.ctx, sourceDenom)
				suite.Require().False(found)
			},
		},
		{
			"failure, multiple denoms map to same contract",
			func() {
				keeper := suite.app.CronosKeeper
				err := keeper.SetAutoContractForDenom(suite.ctx, denom1, autoContract)
				suite.Require().NoError(err)
				err = keeper.SetExternalContractForDenom(suite.ctx, denom2, autoContract)
				suite.Require().Error(err)
			},
		},
		{
			"failure, multiple denoms map to same auto contract",
			func() {
				keeper := suite.app.CronosKeeper
				err := keeper.SetAutoContractForDenom(suite.ctx, denom1, autoContract)
				suite.Require().NoError(err)
				err = keeper.SetAutoContractForDenom(suite.ctx, denom2, autoContract)
				suite.Require().Error(err)
			},
		},
		{
			"failure, multiple denoms map to same external contract",
			func() {
				keeper := suite.app.CronosKeeper
				err := keeper.SetExternalContractForDenom(suite.ctx, denom1, externalContract)
				suite.Require().NoError(err)
				err = keeper.SetExternalContractForDenom(suite.ctx, denom2, externalContract)
				suite.Require().Error(err)
			},
		},
		{
			"failure, SetExternal rejects mapped denom and keeps reverse entry owned by another denom",
			func() {
				// Bug-1 regression: SetExternalContractForDenom must not delete
				// ContractToDenomKey(auto) when that entry already belongs to denom2.
				keeper := suite.app.CronosKeeper
				store := suite.ctx.KVStore(suite.app.GetKey(types.StoreKey))

				legacyDenom := denom + "legacy2"
				legacyAuto := common.BigToAddress(big.NewInt(7))
				denom2External := common.BigToAddress(big.NewInt(9))

				// Corrupted state: legacyDenom auto points to legacyAuto, but reverse entry
				// is already owned by denom2.
				store.Set(types.DenomToAutoContractKey(legacyDenom), legacyAuto.Bytes())
				store.Set(types.DenomToExternalContractKey(denom2), legacyAuto.Bytes())
				store.Set(types.ContractToDenomKey(legacyAuto.Bytes()), []byte(denom2))

				// Set a new external for legacyDenom; the auto cleanup must NOT delete
				// ContractToDenomKey(legacyAuto) since it now belongs to denom2.
				err := keeper.SetExternalContractForDenom(suite.ctx, legacyDenom, denom2External)
				suite.Require().Error(err)

				// denom2's mapping must still be intact.
				contract, found := keeper.GetContractByDenom(suite.ctx, denom2)
				suite.Require().True(found)
				suite.Require().Equal(legacyAuto, contract)
				denomByLegacyAuto, found := keeper.GetDenomByContract(suite.ctx, legacyAuto)
				suite.Require().True(found)
				suite.Require().Equal(denom2, denomByLegacyAuto)
			},
		},
		{
			"success, DeleteExternal does not delete reverse entry owned by another denom",
			func() {
				// Bug-1 regression: DeleteExternalContractForDenom must not delete
				// ContractToDenomKey(auto) when that entry already belongs to denom2.
				keeper := suite.app.CronosKeeper
				store := suite.ctx.KVStore(suite.app.GetKey(types.StoreKey))

				legacyDenom := denom + "legacy3"
				legacyAuto := common.BigToAddress(big.NewInt(10))
				legacyExternal := common.BigToAddress(big.NewInt(11))

				// Corrupted state: legacyDenom auto points to legacyAuto, but reverse entry
				// is already owned by denom2.
				store.Set(types.DenomToAutoContractKey(legacyDenom), legacyAuto.Bytes())
				store.Set(types.DenomToExternalContractKey(legacyDenom), legacyExternal.Bytes())
				store.Set(types.ContractToDenomKey(legacyExternal.Bytes()), []byte(legacyDenom))
				store.Set(types.DenomToExternalContractKey(denom2), legacyAuto.Bytes())
				store.Set(types.ContractToDenomKey(legacyAuto.Bytes()), []byte(denom2))

				// Delete legacyDenom's external. The auto cleanup must NOT delete
				// ContractToDenomKey(legacyAuto) since it now belongs to denom2.
				deleted := keeper.DeleteExternalContractForDenom(suite.ctx, legacyDenom)
				suite.Require().True(deleted)

				// legacyDenom should not retain a conflicting auto mapping.
				_, found := keeper.GetContractByDenom(suite.ctx, legacyDenom)
				suite.Require().False(found)

				// denom2's mapping must still be intact.
				contract, found := keeper.GetContractByDenom(suite.ctx, denom2)
				suite.Require().True(found)
				suite.Require().Equal(legacyAuto, contract)
				denomByLegacyAuto, found := keeper.GetDenomByContract(suite.ctx, legacyAuto)
				suite.Require().True(found)
				suite.Require().Equal(denom2, denomByLegacyAuto)
			},
		},
		{
			"success, DeleteExternal fixes stale reverse entry owned by no one",
			func() {
				keeper := suite.app.CronosKeeper
				store := suite.ctx.KVStore(suite.app.GetKey(types.StoreKey))

				legacyDenom := denom + "legacy4"
				legacyAuto := common.BigToAddress(big.NewInt(12))
				legacyExternal := common.BigToAddress(big.NewInt(13))
				staleDenom := denom + "stale"

				// Corrupted state: legacyDenom auto points to legacyAuto,
				// but reverse entry points to staleDenom which owns nothing.
				store.Set(types.DenomToAutoContractKey(legacyDenom), legacyAuto.Bytes())
				store.Set(types.DenomToExternalContractKey(legacyDenom), legacyExternal.Bytes())
				store.Set(types.ContractToDenomKey(legacyExternal.Bytes()), []byte(legacyDenom))
				store.Set(types.ContractToDenomKey(legacyAuto.Bytes()), []byte(staleDenom))

				deleted := keeper.DeleteExternalContractForDenom(suite.ctx, legacyDenom)
				suite.Require().True(deleted)

				// Reverse entry should be repaired to legacyDenom.
				denomByLegacyAuto, found := keeper.GetDenomByContract(suite.ctx, legacyAuto)
				suite.Require().True(found)
				suite.Require().Equal(legacyDenom, denomByLegacyAuto)

				contract, found := keeper.GetContractByDenom(suite.ctx, legacyDenom)
				suite.Require().True(found)
				suite.Require().Equal(legacyAuto, contract)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.malleate()
		})
	}
}

func (suite *KeeperTestSuite) MintCoinsToModule(module string, coins sdk.Coins) error {
	err := suite.app.BankKeeper.MintCoins(suite.ctx, module, coins)
	if err != nil {
		return err
	}
	return nil
}

func (suite *KeeperTestSuite) GetBalance(address sdk.AccAddress, denom string) sdk.Coin {
	return suite.app.BankKeeper.GetBalance(suite.ctx, address, denom)
}

func (suite *KeeperTestSuite) TestOnRecvVouchers() {
	privKey, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	address := sdk.AccAddress(privKey.PubKey().Address())

	testCases := []struct {
		name      string
		coins     sdk.Coins
		malleate  func()
		postCheck func()
		expErr    bool
	}{
		{
			"state reverted after error",
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123)), sdk.NewCoin("bad", sdkmath.NewInt(10))),
			func() {
				err := suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				// Verify balance IBC coin pre operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin pre operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(0), evmCoin.Amount)
			},
			func() {
				// Verify balance IBC coin post operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin post operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(0), evmCoin.Amount)
			},
			true,
		},
		{
			"state committed upon success",
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))),
			func() {
				err := suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				// Verify balance IBC coin pre operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin pre operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(0), evmCoin.Amount)
			},
			func() {
				// Verify balance IBC coin post operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(0), ibcCroCoin.Amount)
				// Verify balance EVM coin post operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(1230000000000), evmCoin.Amount)
			},
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			// Create Cronos Keeper with mock transfer keeper
			cronosKeeper := *cronosmodulekeeper.NewKeeper(
				suite.app.EncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.EvmKeeper,
				suite.app.AccountKeeper,
				authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			)
			suite.app.CronosKeeper = cronosKeeper

			tc.malleate()
			err := suite.app.CronosKeeper.OnRecvVouchers(suite.ctx, tc.coins, address.String())
			if tc.expErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
			tc.postCheck()
		})
	}
}

func (suite *KeeperTestSuite) TestRegisterOrUpdateTokenMapping() {
	contractAddress := "0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883"

	testCases := []struct {
		name        string
		msg         types.MsgUpdateTokenMapping
		malleate    func(msg *types.MsgUpdateTokenMapping)
		error       bool
		deploy      bool
		denomPrefix string
		postCheck   func(msg *types.MsgUpdateTokenMapping)
	}{
		{
			"Non source token, no error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "",
				Contract: "",
				Symbol:   "",
				Decimal:  0,
			},
			nil,
			false,
			true,
			"gravity",
			nil,
		},
		{
			"Non source token, no code, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "gravity0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883",
				Contract: contractAddress,
				Symbol:   "",
				Decimal:  0,
			},
			nil,
			true,
			false,
			"",
			nil,
		},
		{
			"No hex contract address, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "gravity0xf6d4fecb1a6fb7c2ca350169a050d483bd87b883",
				Contract: "test",
				Symbol:   "",
				Decimal:  0,
			},
			nil,
			true,
			false,
			"",
			nil,
		},
		{
			"Non source token, no hex contract address, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xtest",
				Contract: "test",
				Symbol:   "",
				Decimal:  0,
			},
			nil,
			true,
			false,
			"",
			nil,
		},
		{
			"Non source token, already exists, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "gravity" + contractAddress,
				Contract: "",
				Symbol:   "",
				Decimal:  0,
			},
			func(msg *types.MsgUpdateTokenMapping) {
				code := []byte{0x1}
				codeHash := ethcrypto.Keccak256Hash(code)
				suite.app.EvmKeeper.SetCode(suite.ctx, codeHash.Bytes(), code)
				existingContract := common.HexToAddress(contractAddress)
				updatedContract := common.BigToAddress(big.NewInt(2))
				err := suite.app.EvmKeeper.SetAccount(suite.ctx, existingContract, statedb.Account{
					Nonce:    0,
					CodeHash: codeHash.Bytes(),
				})
				suite.Require().NoError(err)
				err = suite.app.EvmKeeper.SetAccount(suite.ctx, updatedContract, statedb.Account{
					Nonce:    0,
					CodeHash: codeHash.Bytes(),
				})
				suite.Require().NoError(err)
				err = suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, msg.Denom, existingContract)
				suite.Require().NoError(err)
				msg.Contract = updatedContract.Hex()
			},
			true,
			false,
			"",
			nil,
		},
		{
			"Source token, invalid denom, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xf6d4fecb1a6fb7c2ca350169a050d483bd87b88@",
				Contract: contractAddress,
				Symbol:   "",
				Decimal:  0,
			},
			nil,
			true,
			false,
			"",
			nil,
		},
		{
			"Source token, no code, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883",
				Contract: contractAddress,
				Symbol:   "",
				Decimal:  0,
			},
			nil,
			true,
			false,
			"",
			nil,
		},
		{
			"Source token, contract mismatch with embedded denom address, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883",
				Contract: "0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b884",
				Symbol:   "",
				Decimal:  0,
			},
			func(msg *types.MsgUpdateTokenMapping) {
				code := []byte{0x1}
				codeHash := ethcrypto.Keccak256Hash(code)
				suite.app.EvmKeeper.SetCode(suite.ctx, codeHash.Bytes(), code)
				replacement := common.HexToAddress(msg.Contract)
				err := suite.app.EvmKeeper.SetAccount(suite.ctx, replacement, statedb.Account{
					Nonce:    0,
					CodeHash: codeHash.Bytes(),
				})
				suite.Require().NoError(err)
			},
			true,
			false,
			"",
			nil,
		},
		{
			"Source token, denom correct, no error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "",
				Contract: "",
				Symbol:   "",
				Decimal:  0,
			},
			nil,
			false,
			true,
			"cronos",
			nil,
		},
		{
			"Source token, invalid contract, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883",
				Contract: "nothex",
				Symbol:   "",
				Decimal:  0,
			},
			nil,
			true,
			false,
			"",
			nil,
		},
		{
			"Source token, denom correct with decimal, no error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "",
				Contract: "",
				Symbol:   "Test",
				Decimal:  6,
			},
			nil,
			false,
			true,
			"cronos",
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			// Create Cronos Keeper with mock transfer keeper
			cronosKeeper := *cronosmodulekeeper.NewKeeper(
				suite.app.EncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.EvmKeeper,
				suite.app.AccountKeeper,
				authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			)
			suite.app.CronosKeeper = cronosKeeper

			msg := tc.msg
			if tc.deploy {
				contract, err := suite.app.CronosKeeper.DeployModuleCRC21(suite.ctx, "Test")
				suite.Require().NoError(err)
				msg.Contract = contract.Hex()
				msg.Denom = tc.denomPrefix + contract.Hex()
			}
			if tc.malleate != nil {
				tc.malleate(&msg)
			}
			err := suite.app.CronosKeeper.RegisterOrUpdateTokenMapping(suite.ctx, &msg)
			if tc.error {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				if tc.postCheck != nil {
					tc.postCheck(&msg)
				}
			}
		})
	}
}
