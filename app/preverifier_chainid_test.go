package app

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ResolvePreVerifierChainIDSuite struct {
	suite.Suite
}

func TestResolvePreVerifierChainIDSuite(t *testing.T) {
	suite.Run(t, new(ResolvePreVerifierChainIDSuite))
}

func (s *ResolvePreVerifierChainIDSuite) TestResolvePreVerifierChainID() {
	testCases := []struct {
		name        string
		flagChainID string
		appChainID  string
		expected    string
	}{
		{
			name:        "flag set overrides app chain ID",
			flagChainID: "cronos_777-1",
			appChainID:  "cronos_777-2",
			expected:    "cronos_777-1",
		},
		{
			name:        "flag empty falls back to app chain ID",
			flagChainID: "",
			appChainID:  "cronos_777-1",
			expected:    "cronos_777-1",
		},
		{
			name:        "both empty stays empty",
			flagChainID: "",
			appChainID:  "",
			expected:    "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Equal(tc.expected, resolvePreVerifierChainID(tc.flagChainID, tc.appChainID))
		})
	}
}
