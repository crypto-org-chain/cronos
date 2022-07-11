package keeper

import (
	"github.com/crypto-org-chain/cronos/x/icactl/types"
)

var _ types.QueryServer = Keeper{}
