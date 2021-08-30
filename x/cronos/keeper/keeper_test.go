package keeper_test

import (
	"math/big"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
	"github.com/tharsis/ethermint/crypto/ethsecp256k1"
	ethermint "github.com/tharsis/ethermint/types"

	"github.com/crypto-org-chain/cronos/app"
)

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

type KeeperTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *app.App
}

func (suite *KeeperTestSuite) DoSetupTest(t require.TestingT) {
	checkTx := false

	// account key
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	address := common.BytesToAddress(priv.PubKey().Address().Bytes())

	// consensus key
	priv, err = ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	consAddress := sdk.ConsAddress(priv.PubKey().Address())

	suite.app = app.Setup(checkTx)
	suite.ctx = suite.app.NewContext(checkTx, tmproto.Header{
		Height:          1,
		ChainID:         app.TestAppChainID,
		Time:            time.Now().UTC(),
		ProposerAddress: consAddress.Bytes(),
		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
		LastBlockId: tmproto.BlockID{
			Hash: tmhash.Sum([]byte("block_id")),
			PartSetHeader: tmproto.PartSetHeader{
				Total: 11,
				Hash:  tmhash.Sum([]byte("partset_header")),
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
	suite.app.EvmKeeper.WithContext(suite.ctx)

	// initialize the validator
	acc := &ethermint.EthAccount{
		BaseAccount: authtypes.NewBaseAccount(sdk.AccAddress(address.Bytes()), nil, 0, 0),
		CodeHash:    common.BytesToHash(ethcrypto.Keccak256(nil)).String(),
	}

	suite.app.AccountKeeper.SetAccount(suite.ctx, acc)

	valAddr := sdk.ValAddress(address.Bytes())
	validator, err := stakingtypes.NewValidator(valAddr, priv.PubKey(), stakingtypes.Description{})
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
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

func (suite *KeeperTestSuite) TestDenomContractMap() {
	suite.SetupTest()
	keeper := suite.app.CronosKeeper

	denom := "testdenom"
	autoContract := common.BigToAddress(big.NewInt(1))
	externalContract := common.BigToAddress(big.NewInt(2))

	contract, found := keeper.GetContractByDenom(suite.ctx, denom)
	suite.Require().False(found)

	keeper.SetAutoContractForDenom(suite.ctx, denom, autoContract)

	contract, found = keeper.GetContractByDenom(suite.ctx, denom)
	suite.Require().True(found)
	suite.Require().Equal(autoContract, contract)

	keeper.SetExternalContractForDenom(suite.ctx, denom, externalContract)

	contract, found = keeper.GetContractByDenom(suite.ctx, denom)
	suite.Require().True(found)
	suite.Require().Equal(externalContract, contract)

}
