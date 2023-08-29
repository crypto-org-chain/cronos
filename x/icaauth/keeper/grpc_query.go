package keeper

import (
	"github.com/crypto-org-chain/cronos/v2/x/icaauth/types"
)

var _ types.QueryServer = Keeper{}
