package app

import (
	"testing"

	"github.com/crypto-org-chain/cronos-store/memiavl"
	"github.com/crypto-org-chain/cronos-store/store/rootmulti"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

type recordingMultiStore struct {
	storetypes.CommitMultiStore

	lastVersion  int64
	call         string
	loadedVer    int64
	loadedUpgrds *storetypes.StoreUpgrades
}

func (m *recordingMultiStore) LastCommitID() storetypes.CommitID {
	return storetypes.CommitID{Version: m.lastVersion}
}

func (m *recordingMultiStore) LoadLatestVersion() error {
	m.call = "LoadLatestVersion"
	return nil
}

func (m *recordingMultiStore) LoadVersion(ver int64) error {
	m.call, m.loadedVer = "LoadVersion", ver
	return nil
}

func (m *recordingMultiStore) LoadLatestVersionAndUpgrade(upgrades *storetypes.StoreUpgrades) error {
	m.call, m.loadedUpgrds = "LoadLatestVersionAndUpgrade", upgrades
	return nil
}

func (m *recordingMultiStore) LoadVersionAndUpgrade(ver int64, upgrades *storetypes.StoreUpgrades) error {
	m.call, m.loadedVer, m.loadedUpgrds = "LoadVersionAndUpgrade", ver, upgrades
	return nil
}

type StoreLoaderTestSuite struct {
	suite.Suite
}

func TestStoreLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(StoreLoaderTestSuite))
}

func (s *StoreLoaderTestSuite) TestMaxVersionUpgradeStoreLoader() {
	deleteParams := &storetypes.StoreUpgrades{Deleted: []string{"params"}}
	testCases := []struct {
		name          string
		maxVersion    int64
		lastVersion   int64
		upgradeHeight int64
		upgrades      *storetypes.StoreUpgrades
		expCall       string
		expVersion    int64
		expUpgrades   *storetypes.StoreUpgrades
		expErr        string
	}{
		{
			name:          "no versiondb, at upgrade height",
			lastVersion:   99,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadLatestVersionAndUpgrade",
			expUpgrades:   deleteParams,
		},
		{
			name:          "no versiondb, not at upgrade height",
			lastVersion:   100,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadLatestVersion",
		},
		{
			name:          "versiondb at the block before upgrade height, store ahead of versiondb",
			maxVersion:    99,
			lastVersion:   105,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersionAndUpgrade",
			expVersion:    99,
			expUpgrades:   deleteParams,
		},
		{
			name:          "versiondb and store both at the block before upgrade height",
			maxVersion:    99,
			lastVersion:   99,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersionAndUpgrade",
			expVersion:    99,
			expUpgrades:   deleteParams,
		},
		{
			name:          "versiondb at the block before upgrade height, store behind versiondb",
			maxVersion:    99,
			lastVersion:   50,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expErr: "store version 50 is behind the block before upgrade height 100 while versiondb is at 99: " +
				"on memiavl, run the previous binary until it halts at the upgrade, then restart; on iavl, restore the store to at least version 99",
		},
		{
			name:          "versiondb past upgrade height, store well before it",
			maxVersion:    105,
			lastVersion:   50,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expErr:        "while versiondb is at 105: on memiavl, run the previous binary until it halts at the upgrade, then restart; on iavl, restore the store to at least version 105",
		},
		{
			name:          "versiondb at the upgrade block, store at the block before it",
			maxVersion:    100,
			lastVersion:   99,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersionAndUpgrade",
			expVersion:    99,
			expUpgrades:   deleteParams,
		},
		{
			name:          "versiondb past upgrade height, store at the block before it",
			maxVersion:    105,
			lastVersion:   99,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersionAndUpgrade",
			expVersion:    99,
			expUpgrades:   deleteParams,
		},
		{
			name:          "versiondb and store both past upgrade height",
			maxVersion:    105,
			lastVersion:   100,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersion",
			expVersion:    105,
		},
		{
			name:          "versiondb lags store at the block before upgrade height",
			maxVersion:    98,
			lastVersion:   99,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expErr:        "versiondb version 98 is behind the block before upgrade height 100: run the previous binary",
		},
		{
			name:          "versiondb lags store already past upgrade height",
			maxVersion:    98,
			lastVersion:   105,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expErr:        "store is already past it at 105: restore versiondb to at least version 99 or resync it",
		},
		{
			name:          "versiondb lags store at the upgrade block itself",
			maxVersion:    98,
			lastVersion:   100,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expErr:        "store is already past it at 100",
		},
		{
			name:          "versiondb lags store, store two blocks before upgrade height",
			maxVersion:    97,
			lastVersion:   98,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersion",
			expVersion:    97,
		},
		{
			name:          "versiondb and store both before upgrade height",
			maxVersion:    50,
			lastVersion:   60,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersion",
			expVersion:    50,
		},
		{
			name:          "store behind versiondb, both before upgrade height",
			maxVersion:    60,
			lastVersion:   50,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersion",
			expVersion:    60,
		},
		{
			name:          "versiondb lags store at the block before upgrade height, no store changes",
			maxVersion:    98,
			lastVersion:   99,
			upgradeHeight: 100,
			upgrades:      &storetypes.StoreUpgrades{},
			expCall:       "LoadVersion",
			expVersion:    98,
		},
		{
			name:          "versiondb at the block before upgrade height, no store changes",
			maxVersion:    99,
			lastVersion:   99,
			upgradeHeight: 100,
			upgrades:      &storetypes.StoreUpgrades{},
			expCall:       "LoadVersion",
			expVersion:    99,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ms := &recordingMultiStore{lastVersion: tc.lastVersion}
			err := MaxVersionUpgradeStoreLoader(tc.maxVersion, tc.upgradeHeight, tc.upgrades)(ms)
			if tc.expErr != "" {
				s.Require().ErrorContains(err, tc.expErr)
			} else {
				s.Require().NoError(err)
			}
			s.Require().Equal(tc.expCall, ms.call)
			s.Require().Equal(tc.expVersion, ms.loadedVer)
			s.Require().Equal(tc.expUpgrades, ms.loadedUpgrds)
		})
	}
}

// Pins the memiavl behavior the loader relies on: before load, LastCommitID reports the version a higher target clamps to.
func (s *StoreLoaderTestSuite) TestMaxVersionUpgradeStoreLoaderOnMemIAVL() {
	const (
		chainID       = "test_chain"
		upgradeHeight = int64(4)
	)
	testCases := []struct {
		name       string
		maxVersion int64
	}{
		{name: "versiondb at the block before upgrade height", maxVersion: upgradeHeight - 1},
		{name: "versiondb at the upgrade block", maxVersion: upgradeHeight},
		{name: "versiondb past upgrade height", maxVersion: upgradeHeight + 5},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			dir := s.T().TempDir()
			db, err := memiavl.Load(dir, memiavl.Options{
				CreateIfMissing:   true,
				InitialStores:     []string{"params", "test"},
				AsyncCommitBuffer: -1,
			}, chainID)
			s.Require().NoError(err)
			for range upgradeHeight - 1 {
				_, err = db.Commit()
				s.Require().NoError(err)
			}
			s.Require().NoError(db.Close())

			ms := rootmulti.NewStore(dir, log.NewNopLogger(), false, false, chainID)
			ms.MountStoreWithDB(storetypes.NewKVStoreKey("test"), storetypes.StoreTypeIAVL, nil)
			upgrades := &storetypes.StoreUpgrades{Deleted: []string{"params"}}
			s.Require().NoError(MaxVersionUpgradeStoreLoader(tc.maxVersion, upgradeHeight, upgrades)(ms))
			s.Require().Equal(upgradeHeight-1, ms.LastCommitID().Version)
			ms.Commit()
			s.Require().NoError(ms.Close())

			db, err = memiavl.Load(dir, memiavl.Options{ReadOnly: true}, chainID)
			s.Require().NoError(err)
			defer db.Close()
			s.Require().Equal(upgradeHeight, db.Version())
			s.Require().Nil(db.TreeByName("params"))
		})
	}
}
