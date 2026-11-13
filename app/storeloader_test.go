package app

import (
	"testing"

	"github.com/stretchr/testify/suite"

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
			name:          "versiondb at upgrade height, store ahead of versiondb",
			maxVersion:    99,
			lastVersion:   105,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersionAndUpgrade",
			expVersion:    99,
			expUpgrades:   deleteParams,
		},
		{
			name:          "versiondb not at upgrade height",
			maxVersion:    100,
			lastVersion:   99,
			upgradeHeight: 100,
			upgrades:      deleteParams,
			expCall:       "LoadVersion",
			expVersion:    100,
		},
		{
			name:          "versiondb at upgrade height, no store changes",
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
			s.Require().NoError(MaxVersionUpgradeStoreLoader(tc.maxVersion, tc.upgradeHeight, tc.upgrades)(ms))
			s.Require().Equal(tc.expCall, ms.call)
			s.Require().Equal(tc.expVersion, ms.loadedVer)
			s.Require().Equal(tc.expUpgrades, ms.loadedUpgrds)
		})
	}
}
