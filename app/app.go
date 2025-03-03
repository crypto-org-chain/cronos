package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"slices"
	"sort"

	"filippo.io/age"

	abci "github.com/cometbft/cometbft/abci/types"
	tmos "github.com/cometbft/cometbft/libs/os"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"
	"github.com/gorilla/mux"
	"github.com/spf13/cast"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"
	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/errors"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"cosmossdk.io/x/evidence"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	mempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	sigtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/mint"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	ibccallbacks "github.com/cosmos/ibc-go/modules/apps/callbacks"
	"github.com/cosmos/ibc-go/modules/capability"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ica "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts"
	icacontroller "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
	ibcfeekeeper "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/keeper"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	ibcclienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	ibcconnectiontypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
	ibctm "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"

	// Force-load the tracer engines to trigger registration
	"github.com/ethereum/go-ethereum/core/vm"
	ethparams "github.com/ethereum/go-ethereum/params"

	evmapp "github.com/evmos/ethermint/app"
	evmante "github.com/evmos/ethermint/app/ante"
	evmenc "github.com/evmos/ethermint/encoding"
	"github.com/evmos/ethermint/ethereum/eip712"
	srvflags "github.com/evmos/ethermint/server/flags"
	ethermint "github.com/evmos/ethermint/types"
	"github.com/evmos/ethermint/x/evm"
	evmkeeper "github.com/evmos/ethermint/x/evm/keeper"
	v0evmtypes "github.com/evmos/ethermint/x/evm/migrations/v0/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/evmos/ethermint/x/feemarket"
	feemarketkeeper "github.com/evmos/ethermint/x/feemarket/keeper"
	feemarkettypes "github.com/evmos/ethermint/x/feemarket/types"

	// this line is used by starport scaffolding # stargate/app/moduleImport

	memiavlstore "github.com/crypto-org-chain/cronos/store"
	"github.com/crypto-org-chain/cronos/v2/client/docs"
	"github.com/crypto-org-chain/cronos/v2/x/cronos"
	cronosclient "github.com/crypto-org-chain/cronos/v2/x/cronos/client"
	cronoskeeper "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	evmhandlers "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/evmhandlers"
	cronosprecompiles "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/precompiles"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/middleware"
	cronostypes "github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	e2eekeyring "github.com/crypto-org-chain/cronos/v2/x/e2ee/keyring"

	e2ee "github.com/crypto-org-chain/cronos/v2/x/e2ee"
	e2eekeeper "github.com/crypto-org-chain/cronos/v2/x/e2ee/keeper"
	e2eetypes "github.com/crypto-org-chain/cronos/v2/x/e2ee/types"
	"github.com/ethereum/go-ethereum/common"

	// force register the extension json-rpc.
	_ "github.com/crypto-org-chain/cronos/v2/x/cronos/rpc"
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"
)

const (
	Name = "cronos"

	// AddrLen is the allowed length (in bytes) for an address.
	//
	// NOTE: In the SDK, the default value is 255.
	AddrLen = 20

	FlagBlockedAddresses             = "blocked-addresses"
	FlagUnsafeIgnoreBlockListFailure = "unsafe-ignore-block-list-failure"
	FlagUnsafeDummyCheckTx           = "unsafe-dummy-check-tx"
)

var Forks = []Fork{}

// this line is used by starport scaffolding # stargate/wasm/app/enabledProposals

func getGovProposalHandlers() []govclient.ProposalHandler {
	var govProposalHandlers []govclient.ProposalHandler
	// this line is used by starport scaffolding # stargate/app/govProposalHandlers

	govProposalHandlers = append(govProposalHandlers,
		paramsclient.ProposalHandler,
		cronosclient.ProposalHandler,
		// this line is used by starport scaffolding # stargate/app/govProposalHandler
	)

	return govProposalHandlers
}

var (
	// DefaultNodeHome default home directories for the application daemon
	DefaultNodeHome string

	// module account permissions
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		ibcfeetypes.ModuleName:         nil,
		icatypes.ModuleName:            nil,
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner}, // used for secure addition and subtraction of balance using module account
		cronostypes.ModuleName:         {authtypes.Minter, authtypes.Burner},
	}
	// Module configurator

	// module accounts that are allowed to receive tokens
	allowedReceivingModAcc = map[string]bool{}
)

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome = filepath.Join(userHomeDir, "."+Name)
}

func StoreKeys() (
	map[string]*storetypes.KVStoreKey,
	map[string]*storetypes.MemoryStoreKey,
	map[string]*storetypes.TransientStoreKey,
	map[string]*storetypes.ObjectStoreKey,
) {
	storeKeys := []string{
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, capabilitytypes.StoreKey, consensusparamtypes.StoreKey,
		feegrant.StoreKey, crisistypes.StoreKey,
		// ibc keys
		ibcexported.StoreKey, ibctransfertypes.StoreKey,
		ibcfeetypes.StoreKey,
		// ica keys
		icacontrollertypes.StoreKey,
		icahosttypes.StoreKey,
		// ethermint keys
		evmtypes.StoreKey, feemarkettypes.StoreKey,
		// e2ee keys
		e2eetypes.StoreKey,
		// this line is used by starport scaffolding # stargate/app/storeKey
		cronostypes.StoreKey,
	}
	keys := storetypes.NewKVStoreKeys(storeKeys...)
	tkeys := storetypes.NewTransientStoreKeys(paramstypes.TStoreKey)
	memKeys := storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
	okeys := storetypes.NewObjectStoreKeys(banktypes.ObjectStoreKey, evmtypes.ObjectStoreKey)

	return keys, memKeys, tkeys, okeys
}

var (
	_ runtime.AppI            = (*App)(nil)
	_ servertypes.Application = (*App)(nil)
)

type GenesisState = map[string]json.RawMessage

// App extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
type App struct {
	*baseapp.BaseApp

	// encoding
	cdc               *codec.LegacyAmino
	txConfig          client.TxConfig
	txDecoder         sdk.TxDecoder
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry

	invCheckPeriod uint

	pendingTxListeners []evmante.PendingTxListener

	// keys to access the substores
	keys    map[string]*storetypes.KVStoreKey
	tkeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey
	okeys   map[string]*storetypes.ObjectStoreKey

	// keepers
	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	CapabilityKeeper      *capabilitykeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	MintKeeper            mintkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	CrisisKeeper          crisiskeeper.Keeper
	UpgradeKeeper         upgradekeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper
	IBCKeeper             *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	IBCFeeKeeper          ibcfeekeeper.Keeper
	ICAControllerKeeper   icacontrollerkeeper.Keeper
	ICAHostKeeper         icahostkeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	TransferKeeper        ibctransferkeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	ConsensusParamsKeeper consensusparamkeeper.Keeper

	// make scoped keepers public for test purposes
	ScopedIBCKeeper           capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper      capabilitykeeper.ScopedKeeper
	ScopedICAControllerKeeper capabilitykeeper.ScopedKeeper

	// Ethermint keepers
	EvmKeeper       *evmkeeper.Keeper
	FeeMarketKeeper feemarketkeeper.Keeper

	// e2ee keeper
	E2EEKeeper e2eekeeper.Keeper

	// this line is used by starport scaffolding # stargate/app/keeperDeclaration

	CronosKeeper cronoskeeper.Keeper

	// the module manager
	ModuleManager      *module.Manager
	BasicModuleManager module.BasicManager

	// simulation manager
	sm *module.SimulationManager

	// module configurator
	configurator module.Configurator

	qms storetypes.RootMultiStore

	blockProposalHandler *ProposalHandler

	// unsafe to set for validator, used for testing
	dummyCheckTx bool
}

// New returns a reference to an initialized chain.
// NewSimApp returns a reference to an initialized SimApp.
func New(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	// this line is used by starport scaffolding # stargate/app/newArgument
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *App {
	encodingConfig := evmenc.MakeConfig()
	appCodec := encodingConfig.Codec
	cdc := encodingConfig.Amino
	txConfig := encodingConfig.TxConfig
	interfaceRegistry := encodingConfig.InterfaceRegistry
	txDecoder := txConfig.TxDecoder()
	eip712.SetEncodingConfig(encodingConfig)

	homePath := cast.ToString(appOpts.Get(flags.FlagHome))
	var identity age.Identity
	{
		if cast.ToString(appOpts.Get("mode")) == "validator" {
			krBackend := cast.ToString(appOpts.Get(flags.FlagKeyringBackend))
			kr, err := e2eekeyring.New("cronosd", krBackend, homePath, os.Stdin)
			if err != nil {
				panic(err)
			}
			bz, err := kr.Get(e2eetypes.DefaultKeyringName)
			if err != nil {
				logger.Error("e2ee identity for validator not found", "error", err)
				identity = noneIdentity{}
			} else {
				identity, err = age.ParseX25519Identity(string(bz))
				if err != nil {
					logger.Error("e2ee identity for validator is invalid", "error", err)
					identity = noneIdentity{}
				}
			}
		}
	}

	addressCodec := authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	var mpool mempool.Mempool
	if maxTxs := cast.ToInt(appOpts.Get(server.FlagMempoolMaxTxs)); maxTxs >= 0 {
		// NOTE we use custom transaction decoder that supports the sdk.Tx interface instead of sdk.StdTx
		// Setup Mempool and Proposal Handlers
		mpool = mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: evmapp.NewEthSignerExtractionAdapter(mempool.NewDefaultSignerExtractionAdapter()),
			MaxTx:           maxTxs,
		})
	} else {
		mpool = mempool.NoOpMempool{}
	}
	blockProposalHandler := NewProposalHandler(txDecoder, identity, addressCodec)
	baseAppOptions = append(baseAppOptions, func(app *baseapp.BaseApp) {
		app.SetMempool(mpool)

		// Re-use the default prepare proposal handler, extend the transaction validation logic
		defaultProposalHandler := baseapp.NewDefaultProposalHandlerFast(mpool, app)
		defaultProposalHandler.SetTxSelector(NewExtTxSelector(
			baseapp.NewDefaultTxSelector(),
			txDecoder,
			blockProposalHandler.ValidateTransaction,
		))

		app.SetPrepareProposal(defaultProposalHandler.PrepareProposalHandler())

		// The default process proposal handler do nothing when the mempool is noop,
		// so we just implement a new one.
		app.SetProcessProposal(blockProposalHandler.ProcessProposalHandler())
	})

	blockSTMEnabled := cast.ToString(appOpts.Get(srvflags.EVMBlockExecutor)) == "block-stm"

	var cacheSize int
	if !blockSTMEnabled {
		// only enable memiavl cache if block-stm is not enabled, because it's not concurrency-safe.
		cacheSize = cast.ToInt(appOpts.Get(memiavlstore.FlagCacheSize))
	}
	baseAppOptions = memiavlstore.SetupMemIAVL(logger, homePath, appOpts, false, false, cacheSize, baseAppOptions)

	// enable optimistic execution
	baseAppOptions = append(baseAppOptions, baseapp.SetOptimisticExecution())

	// NOTE we use custom transaction decoder that supports the sdk.Tx interface instead of sdk.StdTx
	bApp := baseapp.NewBaseApp(Name, logger, db, txConfig.TxDecoder(), baseAppOptions...)

	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	keys, memKeys, tkeys, okeys := StoreKeys()

	invCheckPeriod := cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod))
	app := &App{
		BaseApp:              bApp,
		cdc:                  cdc,
		txConfig:             txConfig,
		txDecoder:            txDecoder,
		appCodec:             appCodec,
		interfaceRegistry:    interfaceRegistry,
		invCheckPeriod:       invCheckPeriod,
		keys:                 keys,
		tkeys:                tkeys,
		okeys:                okeys,
		memKeys:              memKeys,
		blockProposalHandler: blockProposalHandler,
		dummyCheckTx:         cast.ToBool(appOpts.Get(FlagUnsafeDummyCheckTx)),
	}

	app.SetDisableBlockGasMeter(true)

	// init params keeper and subspaces
	app.ParamsKeeper = initParamsKeeper(appCodec, cdc, keys[paramstypes.StoreKey], tkeys[paramstypes.TStoreKey])

	// get authority address
	authAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// set the BaseApp's parameter store
	app.ConsensusParamsKeeper = consensusparamkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[consensusparamtypes.StoreKey]),
		authAddr,
		runtime.EventService{},
	)
	bApp.SetParamStore(app.ConsensusParamsKeeper.ParamsStore)

	// add capability keeper and ScopeToModule for ibc module
	app.CapabilityKeeper = capabilitykeeper.NewKeeper(appCodec, keys[capabilitytypes.StoreKey], memKeys[capabilitytypes.MemStoreKey])

	// grant capabilities for the ibc and ibc-transfer modules
	scopedIBCKeeper := app.CapabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	scopedTransferKeeper := app.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	scopedICAControllerKeeper := app.CapabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)
	scopedICAHostKeeper := app.CapabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)

	// Applications that wish to enforce statically created ScopedKeepers should call `Seal` after creating
	// their scoped modules in `NewApp` with `ScopeToModule`
	app.CapabilityKeeper.Seal()

	// this line is used by starport scaffolding # stargate/app/scopedKeeper

	// add keepers
	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		ethermint.ProtoAccount,
		maccPerms,
		addressCodec,
		sdk.GetConfig().GetBech32AccountAddrPrefix(),
		authAddr,
	)
	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		okeys[banktypes.ObjectStoreKey],
		app.AccountKeeper,
		app.BlockedAddrs(),
		authAddr,
		logger,
	)
	// optional: enable sign mode textual by overwriting the default tx config (after setting the bank keeper)
	enabledSignModes := slices.Clone(authtx.DefaultSignModes)
	enabledSignModes = append(enabledSignModes, sigtypes.SignMode_SIGN_MODE_TEXTUAL)
	txConfigOpts := authtx.ConfigOptions{
		EnabledSignModes:           enabledSignModes,
		TextualCoinMetadataQueryFn: txmodule.NewBankKeeperCoinMetadataQueryFn(app.BankKeeper),
	}
	txConfig, err := authtx.NewTxConfigWithOptions(
		appCodec,
		txConfigOpts,
	)
	if err != nil {
		panic(err)
	}
	app.txConfig = txConfig
	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		authAddr,
		address.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		address.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	)
	app.MintKeeper = mintkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[minttypes.StoreKey]),
		app.StakingKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.FeeCollectorName,
		authAddr,
	)
	app.DistrKeeper = distrkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[distrtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		authtypes.FeeCollectorName,
		authAddr,
	)
	app.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		app.LegacyAmino(),
		runtime.NewKVStoreService(keys[slashingtypes.StoreKey]),
		app.StakingKeeper,
		authAddr,
	)
	app.CrisisKeeper = *crisiskeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[crisistypes.StoreKey]),
		invCheckPeriod,
		app.BankKeeper,
		authtypes.FeeCollectorName,
		authAddr,
		app.AccountKeeper.AddressCodec(),
	)
	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[feegrant.StoreKey]),
		app.AccountKeeper,
	)

	// get skipUpgradeHeights from the app options
	skipUpgradeHeights := map[int64]bool{}
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}
	// set the governance module account as the authority for conducting upgrades
	app.UpgradeKeeper = *upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		runtime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		appCodec,
		homePath,
		app.BaseApp,
		authAddr,
	)

	// register the staking hooks
	app.StakingKeeper.SetHooks(stakingtypes.NewMultiStakingHooks(
		app.DistrKeeper.Hooks(),
		app.SlashingKeeper.Hooks(),
	))

	// ... other modules keepers

	// Create IBC Keeper
	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec,
		keys[ibcexported.StoreKey],
		app.GetSubspace(ibcexported.ModuleName),
		app.StakingKeeper,
		app.UpgradeKeeper,
		scopedIBCKeeper,
		authAddr,
	)

	// IBC Fee Module keeper
	app.IBCFeeKeeper = ibcfeekeeper.NewKeeper(
		appCodec, keys[ibcfeetypes.StoreKey],
		app.IBCKeeper.ChannelKeeper, // may be replaced with IBC middleware
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.PortKeeper, app.AccountKeeper, app.BankKeeper,
	)

	// ICA Controller keeper
	app.ICAControllerKeeper = icacontrollerkeeper.NewKeeper(
		appCodec, keys[icacontrollertypes.StoreKey], app.GetSubspace(icacontrollertypes.SubModuleName),
		app.IBCFeeKeeper, // use ics29 fee as ics4Wrapper in middleware stack
		app.IBCKeeper.ChannelKeeper, app.IBCKeeper.PortKeeper,
		scopedICAControllerKeeper, app.MsgServiceRouter(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	app.ICAHostKeeper = icahostkeeper.NewKeeper(
		appCodec, keys[icahosttypes.StoreKey], nil,
		app.IBCFeeKeeper, // ISC4 Wrapper: fee IBC middleware
		app.IBCKeeper.ChannelKeeper, app.IBCKeeper.PortKeeper,
		app.AccountKeeper, scopedICAHostKeeper, app.MsgServiceRouter(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	app.ICAHostKeeper.WithQueryRouter(app.GRPCQueryRouter())
	icaModule := ica.NewAppModule(&app.ICAControllerKeeper, &app.ICAHostKeeper)

	// Create Transfer Keepers
	app.TransferKeeper = ibctransferkeeper.NewKeeper(
		appCodec, keys[ibctransfertypes.StoreKey], app.GetSubspace(ibctransfertypes.ModuleName),
		app.IBCFeeKeeper, // ISC4 Wrapper: fee IBC middleware
		app.IBCKeeper.ChannelKeeper, app.IBCKeeper.PortKeeper,
		app.AccountKeeper, app.BankKeeper, scopedTransferKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	// Create Ethermint keepers
	tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))

	// Create Ethermint keepers
	feeMarketS := app.GetSubspace(feemarkettypes.ModuleName)
	app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
		appCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		keys[feemarkettypes.StoreKey],
		feeMarketS,
	)
	// Set authority to x/gov module account to only expect the module account to update params
	evmS := app.GetSubspace(evmtypes.ModuleName)

	gasConfig := storetypes.TransientGasConfig()
	app.EvmKeeper = evmkeeper.NewKeeper(
		appCodec,
		keys[evmtypes.StoreKey], okeys[evmtypes.ObjectStoreKey], authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.FeeMarketKeeper,
		tracer,
		evmS,
		[]evmkeeper.CustomContractFn{
			func(_ sdk.Context, rules ethparams.Rules) vm.PrecompiledContract {
				return cronosprecompiles.NewRelayerContract(app.IBCKeeper, app.IBCFeeKeeper, appCodec, rules, app.Logger())
			},
			func(ctx sdk.Context, rules ethparams.Rules) vm.PrecompiledContract {
				return cronosprecompiles.NewIcaContract(ctx, app.ICAControllerKeeper, &app.CronosKeeper, appCodec, gasConfig)
			},
		},
	)

	// this line is used by starport scaffolding # stargate/app/keeperDefinition

	app.CronosKeeper = *cronoskeeper.NewKeeper(
		appCodec,
		keys[cronostypes.StoreKey],
		keys[cronostypes.MemStoreKey],
		app.BankKeeper,
		app.TransferKeeper,
		app.EvmKeeper,
		app.AccountKeeper,
		authAddr,
	)
	cronosModule := cronos.NewAppModule(app.CronosKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(cronostypes.ModuleName))

	// register the proposal types
	govRouter := govv1beta1.NewRouter()
	govRouter.AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler).
		AddRoute(paramproposal.RouterKey, params.NewParamChangeProposalHandler(app.ParamsKeeper)).
		AddRoute(cronostypes.RouterKey, cronos.NewTokenMappingChangeProposalHandler(app.CronosKeeper))

	govConfig := govtypes.DefaultConfig()
	/*
		Example of setting gov params:
		govConfig.MaxMetadataLen = 10000
	*/

	// set the middleware
	transferModule := transfer.NewAppModule(app.TransferKeeper)
	feeModule := ibcfee.NewAppModule(app.IBCFeeKeeper)

	var transferStack porttypes.IBCModule
	transferStack = transfer.NewIBCModule(app.TransferKeeper)
	transferStack = middleware.NewIBCConversionModule(transferStack, app.CronosKeeper)
	transferStack = ibcfee.NewIBCMiddleware(transferStack, app.IBCFeeKeeper)

	govKeeper := govkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[govtypes.StoreKey]),
		app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.DistrKeeper,
		app.MsgServiceRouter(), govConfig, authAddr,
	)

	// Set legacy router for backwards compatibility with gov v1beta1
	govKeeper.SetLegacyRouter(govRouter)

	app.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(
		// register the governance hooks
		),
	)

	app.EvmKeeper.SetHooks(cronoskeeper.NewLogProcessEvmHook(
		evmhandlers.NewSendToAccountHandler(app.BankKeeper, app.CronosKeeper),
		evmhandlers.NewSendToIbcHandler(app.BankKeeper, app.CronosKeeper),
		evmhandlers.NewSendCroToIbcHandler(app.BankKeeper, app.CronosKeeper),
		evmhandlers.NewSendToIbcV2Handler(app.BankKeeper, app.CronosKeeper),
	))

	var icaControllerStack porttypes.IBCModule
	icaControllerStack = icacontroller.NewIBCMiddleware(nil, app.ICAControllerKeeper)
	icaControllerStack = ibcfee.NewIBCMiddleware(icaControllerStack, app.IBCFeeKeeper)
	// Since the callbacks middleware itself is an ics4wrapper, it needs to be passed to the ica controller keeper
	ics4Wrapper := icaControllerStack.(porttypes.Middleware)
	app.ICAControllerKeeper.WithICS4Wrapper(ics4Wrapper)
	// we don't limit gas usage here, because the cronos keeper will use network parameter to control it.
	icaControllerStack = ibccallbacks.NewIBCMiddleware(icaControllerStack, app.IBCFeeKeeper, app.CronosKeeper, math.MaxUint64)

	var icaHostStack porttypes.IBCModule
	icaHostStack = icahost.NewIBCModule(app.ICAHostKeeper)
	icaHostStack = ibcfee.NewIBCMiddleware(icaHostStack, app.IBCFeeKeeper)

	// Create static IBC router, add transfer route, then set and seal it
	ibcRouter := porttypes.NewRouter()
	// Add controller & ica auth modules to IBC router
	ibcRouter.
		AddRoute(icacontrollertypes.SubModuleName, icaControllerStack).
		AddRoute(icahosttypes.SubModuleName, icaHostStack).
		AddRoute(ibctransfertypes.ModuleName, transferStack)

	// this line is used by starport scaffolding # ibc/app/router
	app.IBCKeeper.SetRouter(ibcRouter)

	// Create evidence Keeper for to register the IBC light client misbehavior evidence route
	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[evidencetypes.StoreKey]),
		app.StakingKeeper, app.SlashingKeeper,
		app.AccountKeeper.AddressCodec(),
		runtime.ProvideCometInfoService(),
	)
	// If evidence needs to be handled for the app, set routes in router here and seal
	app.EvidenceKeeper = *evidenceKeeper

	app.E2EEKeeper = e2eekeeper.NewKeeper(keys[e2eetypes.StoreKey], app.AccountKeeper.AddressCodec())

	/****  Module Options ****/

	// NOTE: we may consider parsing `appOpts` inside module constructors. For the moment
	// we prefer to be more strict in what arguments the modules expect.
	skipGenesisInvariants := cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.

	app.ModuleManager = module.NewManager(
		// SDK app modules
		genutil.NewAppModule(
			app.AccountKeeper, app.StakingKeeper, app,
			txConfig,
		),
		auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, app.GetSubspace(banktypes.ModuleName)),
		capability.NewAppModule(appCodec, *app.CapabilityKeeper, false),
		crisis.NewAppModule(&app.CrisisKeeper, skipGenesisInvariants, app.GetSubspace(crisistypes.ModuleName)),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(govtypes.ModuleName)),
		mint.NewAppModule(appCodec, app.MintKeeper, app.AccountKeeper, nil, app.GetSubspace(minttypes.ModuleName)),
		slashing.NewAppModule(
			appCodec,
			app.SlashingKeeper,
			app.AccountKeeper,
			app.BankKeeper,
			app.StakingKeeper,
			app.GetSubspace(slashingtypes.ModuleName),
			app.interfaceRegistry,
		),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(distrtypes.ModuleName)),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName)),
		upgrade.NewAppModule(&app.UpgradeKeeper, app.AccountKeeper.AddressCodec()),
		evidence.NewAppModule(app.EvidenceKeeper),
		params.NewAppModule(app.ParamsKeeper),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),

		// ibc modules
		ibc.NewAppModule(app.IBCKeeper),
		ibctm.AppModule{},
		transferModule,
		icaModule,
		feeModule,

		// Ethermint app modules
		feemarket.NewAppModule(app.FeeMarketKeeper, feeMarketS),
		evm.NewAppModule(app.EvmKeeper, app.AccountKeeper, evmS),
		e2ee.NewAppModule(app.E2EEKeeper),

		// Cronos app modules
		cronosModule,
	)

	// BasicModuleManager defines the module BasicManager is in charge of setting up basic,
	// non-dependant module elements, such as codec registration and genesis verification.
	// By default it is composed of all the module from the module manager.
	// Additionally, app module basics can be overwritten by passing them as argument.
	app.BasicModuleManager = module.NewBasicManagerFromManager(
		app.ModuleManager,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			govtypes.ModuleName:     gov.NewAppModuleBasic(getGovProposalHandlers()),
		})
	app.BasicModuleManager.RegisterLegacyAminoCodec(cdc)
	app.BasicModuleManager.RegisterInterfaces(interfaceRegistry)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0
	beginBlockersOrder := []string{
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		feemarkettypes.ModuleName,
		evmtypes.ModuleName,
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		ibcfeetypes.ModuleName,
		icatypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		govtypes.ModuleName,
		crisistypes.ModuleName,
		genutiltypes.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName,
		vestingtypes.ModuleName,
		cronostypes.ModuleName,
		consensusparamtypes.ModuleName,
	}
	endBlockersOrder := []string{
		crisistypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		ibcfeetypes.ModuleName,
		icatypes.ModuleName,
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		cronostypes.ModuleName,
		consensusparamtypes.ModuleName,
	}

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// NOTE: The genutils module must also occur after auth so that it can access the params from auth.
	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	initGenesisOrder := []string{
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		minttypes.ModuleName,
		ibcexported.ModuleName,
		// evm module denomination is used by the feemarket module, in AnteHandle
		evmtypes.ModuleName,
		// NOTE: feemarket need to be initialized before genutil module:
		// gentx transactions use MinGasPriceDecorator.AnteHandle
		feemarkettypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibcfeetypes.ModuleName,
		icatypes.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		cronostypes.ModuleName,
		consensusparamtypes.ModuleName,
		// NOTE: crisis module must go at the end to check for invariants on each module
		crisistypes.ModuleName,
		e2eetypes.ModuleName,
	}

	app.ModuleManager.SetOrderPreBlockers(
		upgradetypes.ModuleName,
	)
	app.ModuleManager.SetOrderBeginBlockers(beginBlockersOrder...)
	app.ModuleManager.SetOrderEndBlockers(endBlockersOrder...)
	app.ModuleManager.SetOrderInitGenesis(initGenesisOrder...)

	// Uncomment if you want to set a custom migration order here.
	// app.mm.SetOrderMigrations(custom order)

	app.ModuleManager.RegisterInvariants(&app.CrisisKeeper)
	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	if err := app.ModuleManager.RegisterServices(app.configurator); err != nil {
		panic(err)
	}

	// wire up the versiondb's `StreamingService` and `MultiStore`.
	if cast.ToBool(appOpts.Get("versiondb.enable")) {
		var err error
		app.qms, err = app.setupVersionDB(homePath, keys, tkeys, memKeys, okeys)
		if err != nil {
			panic(err)
		}
	}

	var qmsVersion int64
	if app.qms != nil {
		qmsVersion = app.qms.LatestVersion()
	}

	// RegisterUpgradeHandlers is used for registering any on-chain upgrades.
	// Make sure it's called after `app.mm` and `app.configurator` are set.
	storeLoaderOverritten := app.RegisterUpgradeHandlers(app.appCodec, qmsVersion)
	if !storeLoaderOverritten {
		// Register the default store loader
		app.SetStoreLoader(MaxVersionStoreLoader(qmsVersion))
	}

	// add test gRPC service for testing gRPC queries in isolation
	// testdata.RegisterQueryServer(app.GRPCQueryRouter(), testdata.QueryImpl{})

	// create the simulation manager and define the order of the modules for deterministic simulations
	//
	// NOTE: this is not required apps that don't use the simulator for fuzz testing
	// transactions
	overrideModules := map[string]module.AppModuleSimulation{
		// Use custom RandomGenesisAccounts so that auth module could create random EthAccounts in genesis state when genesis.json not specified
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.ModuleManager.Modules, overrideModules)

	autocliv1.RegisterQueryServer(app.GRPCQueryRouter(), runtimeservices.NewAutoCLIQueryService(app.ModuleManager.Modules))

	reflectionSvc, err := runtimeservices.NewReflectionService()
	if err != nil {
		panic(err)
	}
	reflectionv1.RegisterReflectionServiceServer(app.GRPCQueryRouter(), reflectionSvc)

	app.sm.RegisterStoreDecoders()

	// initialize stores
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	app.MountMemoryStores(memKeys)
	app.MountObjectStores(okeys)

	// initialize BaseApp
	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(app.PreBlocker)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)
	if err := app.setAnteHandler(txConfig,
		cast.ToUint64(appOpts.Get(srvflags.EVMMaxTxGasWanted)),
		cast.ToStringSlice(appOpts.Get(FlagBlockedAddresses)),
	); err != nil {
		panic(err)
	}
	// In v0.46, the SDK introduces _postHandlers_. PostHandlers are like
	// antehandlers, but are run _after_ the `runMsgs` execution. They are also
	// defined as a chain, and have the same signature as antehandlers.
	//
	// In baseapp, postHandlers are run in the same store branch as `runMsgs`,
	// meaning that both `runMsgs` and `postHandler` state will be committed if
	// both are successful, and both will be reverted if any of the two fails.
	//
	// The SDK exposes a default empty postHandlers chain.
	//
	// Please note that changing any of the anteHandler or postHandler chain is
	// likely to be a state-machine breaking change, which needs a coordinated
	// upgrade.
	app.setPostHandler()

	// At startup, after all modules have been registered, check that all prot
	// annotations are correct.
	protoFiles, err := proto.MergedRegistry()
	if err != nil {
		panic(err)
	}
	err = msgservice.ValidateProtoAnnotations(protoFiles)
	if err != nil {
		// Once we switch to using protoreflect-based antehandlers, we might
		// want to panic here instead of logging a warning.
		fmt.Fprintln(os.Stderr, err.Error())
	}

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			tmos.Exit(err.Error())
		}

		if qmsVersion > 0 {
			// it should not happens since we constraint the loaded iavl version to not exceed the versiondb version,
			// still keep the check for safety.
			iavlVersion := app.LastBlockHeight()
			if qmsVersion < iavlVersion {
				// try to prevent gap being created in versiondb
				tmos.Exit(fmt.Sprintf("versiondb version %d lag behind iavl version %d", qmsVersion, iavlVersion))
			}
		}

		if err := app.RefreshBlockList(app.NewUncachedContext(false, cmtproto.Header{})); err != nil {
			if !cast.ToBool(appOpts.Get(FlagUnsafeIgnoreBlockListFailure)) {
				panic(err)
			}

			// otherwise, just emit error log
			app.Logger().Error("failed to update blocklist", "error", err)
		}
	}

	app.ScopedIBCKeeper = scopedIBCKeeper
	app.ScopedTransferKeeper = scopedTransferKeeper
	app.ScopedICAControllerKeeper = scopedICAControllerKeeper
	// this line is used by starport scaffolding # stargate/app/beforeInitReturn

	if blockSTMEnabled {
		sdk.SetAddrCacheEnabled(false)
		workers := cast.ToInt(appOpts.Get(srvflags.EVMBlockSTMWorkers))
		if workers == 0 {
			workers = maxParallelism()
		}
		preEstimate := cast.ToBool(appOpts.Get(srvflags.EVMBlockSTMPreEstimate))
		logger.Info("block-stm executor enabled", "workers", workers, "pre-estimate", preEstimate)
		app.SetTxExecutor(evmapp.STMTxExecutor(app.GetStoreKeys(), workers, preEstimate, app.EvmKeeper, txConfig.TxDecoder()))
	} else {
		app.SetTxExecutor(evmapp.DefaultTxExecutor)
	}

	return app
}

// use Ethermint's custom AnteHandler
func (app *App) setAnteHandler(txConfig client.TxConfig, maxGasWanted uint64, blacklist []string) error {
	if len(blacklist) > 0 {
		sort.Strings(blacklist)
		// hash blacklist concatenated
		h := sha256.New()
		for _, addr := range blacklist {
			_, err := h.Write([]byte(addr))
			if err != nil {
				panic(err)
			}
		}
		app.Logger().Error("Setting ante handler with blacklist", "size", len(blacklist), "hash", hex.EncodeToString(h.Sum(nil)))
		for _, addr := range blacklist {
			app.Logger().Error("Blacklisted address", "address", addr)
		}
	} else {
		app.Logger().Error("Setting ante handler without blacklist")
	}
	blockedMap := make(map[string]struct{}, len(blacklist))
	for _, str := range blacklist {
		addr, err := sdk.AccAddressFromBech32(str)
		if err != nil {
			return fmt.Errorf("invalid bech32 address: %s, err: %w", str, err)
		}

		blockedMap[addr.String()] = struct{}{}
	}
	blockAddressDecorator := NewBlockAddressesDecorator(blockedMap, app.CronosKeeper.GetParams)
	options := evmante.HandlerOptions{
		AccountKeeper:          app.AccountKeeper,
		BankKeeper:             app.BankKeeper,
		FeegrantKeeper:         app.FeeGrantKeeper,
		IBCKeeper:              app.IBCKeeper,
		EvmKeeper:              app.EvmKeeper,
		FeeMarketKeeper:        app.FeeMarketKeeper,
		SignModeHandler:        txConfig.SignModeHandler(),
		SigGasConsumer:         evmante.DefaultSigVerificationGasConsumer,
		MaxTxGasWanted:         maxGasWanted,
		ExtensionOptionChecker: ethermint.HasDynamicFeeExtensionOption,
		DynamicFeeChecker:      true,
		DisabledAuthzMsgs: []string{
			sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
			sdk.MsgTypeURL(&vestingtypes.MsgCreateVestingAccount{}),
			sdk.MsgTypeURL(&vestingtypes.MsgCreatePermanentLockedAccount{}),
			sdk.MsgTypeURL(&vestingtypes.MsgCreatePeriodicVestingAccount{}),
		},
		ExtraDecorators:   []sdk.AnteDecorator{blockAddressDecorator},
		PendingTxListener: app.onPendingTx,
	}

	anteHandler, err := evmante.NewAnteHandler(options)
	if err != nil {
		return err
	}

	app.SetAnteHandler(anteHandler)
	return nil
}

func (app *App) setPostHandler() {
	postHandler, err := posthandler.NewPostHandler(
		posthandler.HandlerOptions{},
	)
	if err != nil {
		panic(err)
	}
	app.SetPostHandler(postHandler)
}

// Name returns the name of the App
func (app *App) Name() string { return app.BaseApp.Name() }

// PreBlocker updates every pre begin block
func (app *App) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.ModuleManager.PreBlock(ctx)
}

// BeginBlocker application updates every begin block
func (app *App) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	BeginBlockForks(ctx, app)
	return app.ModuleManager.BeginBlock(ctx)
}

// EndBlocker application updates every end block
func (app *App) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	rsp, err := app.ModuleManager.EndBlock(ctx)
	if err := app.RefreshBlockList(ctx); err != nil {
		app.Logger().Error("failed to update blocklist", "error", err)
	}
	return rsp, err
}

func (app *App) RefreshBlockList(ctx sdk.Context) error {
	// refresh blocklist
	return app.blockProposalHandler.SetBlockList(app.CronosKeeper.GetBlockList(ctx))
}

// InitChainer application update at chain initialization
func (app *App) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}
	if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap()); err != nil {
		return nil, err
	}
	return app.ModuleManager.InitGenesis(ctx, app.appCodec, genesisState)
}

// LoadHeight loads a particular height
func (app *App) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *App) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// BlockedAddrs returns all the app's module account addresses that are not
// allowed to receive external tokens.
func (app *App) BlockedAddrs() map[string]bool {
	blockedAddrs := make(map[string]bool)
	for acc := range maccPerms {
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = !allowedReceivingModAcc[acc]
	}

	return blockedAddrs
}

// LegacyAmino returns SimApp's amino codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *App) LegacyAmino() *codec.LegacyAmino {
	return app.cdc
}

// AppCodec returns your app's codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *App) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns your app's InterfaceRegistry
func (app *App) InterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

func (app *App) EncodingConfig() ethermint.EncodingConfig {
	return ethermint.EncodingConfig{
		InterfaceRegistry: app.InterfaceRegistry(),
		Codec:             app.AppCodec(),
		TxConfig:          app.TxConfig(),
		Amino:             app.LegacyAmino(),
	}
}

// GetKey returns the KVStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *App) GetKey(storeKey string) *storetypes.KVStoreKey {
	return app.keys[storeKey]
}

// GetStoreKeys returns all the stored store keys.
func (app *App) GetStoreKeys() []storetypes.StoreKey {
	keys := make([]storetypes.StoreKey, 0, len(app.keys))
	for _, key := range app.keys {
		keys = append(keys, key)
	}
	for _, key := range app.tkeys {
		keys = append(keys, key)
	}
	for _, key := range app.memKeys {
		keys = append(keys, key)
	}
	for _, key := range app.okeys {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].Name() < keys[j].Name() })
	return keys
}

// GetTKey returns the TransientStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *App) GetTKey(storeKey string) *storetypes.TransientStoreKey {
	return app.tkeys[storeKey]
}

// GetMemKey returns the MemStoreKey for the provided mem key.
//
// NOTE: This is solely used for testing purposes.
func (app *App) GetMemKey(storeKey string) *storetypes.MemoryStoreKey {
	return app.memKeys[storeKey]
}

// GetSubspace returns a param subspace for a given module name.
//
// NOTE: This is solely to be used for testing purposes.
func (app *App) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// SimulationManager implements the SimulationApp interface
func (app *App) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *App) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	app.EvmKeeper.WithChainIDString(clientCtx.ChainID)
	// Register new tx routes from grpc-gateway.
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register new tendermint queries routes from grpc-gateway.
	cmtservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register node gRPC service for grpc-gateway.
	node.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register grpc-gateway routes for all modules.
	app.BasicModuleManager.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register swagger API from root so that other applications can override easily
	if apiConfig.Swagger {
		RegisterSwaggerAPI(clientCtx, apiSvr.Router)
	}
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *App) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *App) RegisterTendermintService(clientCtx client.Context) {
	cmtservice.RegisterTendermintService(
		clientCtx,
		app.BaseApp.GRPCQueryRouter(),
		app.interfaceRegistry,
		app.Query,
	)
}

func (app *App) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	node.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

// DefaultGenesis returns a default genesis from the registered AppModuleBasic's.
func (a *App) DefaultGenesis() map[string]json.RawMessage {
	return a.BasicModuleManager.DefaultGenesis(a.appCodec)
}

func (app *App) TxConfig() client.TxConfig {
	return app.txConfig
}

// AutoCliOpts returns the autocli options for the app.
func (app *App) AutoCliOpts() autocli.AppOptions {
	modules := make(map[string]appmodule.AppModule, 0)
	for _, m := range app.ModuleManager.Modules {
		if moduleWithName, ok := m.(module.HasName); ok {
			moduleName := moduleWithName.Name()
			if appModule, ok := moduleWithName.(appmodule.AppModule); ok {
				modules[moduleName] = appModule
			}
		}
	}
	return autocli.AppOptions{
		Modules:               modules,
		ModuleOptions:         runtimeservices.ExtractAutoCLIOptions(app.ModuleManager.Modules),
		AddressCodec:          authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		ValidatorAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		ConsensusAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	}
}

// RegisterPendingTxListener is used by json-rpc server to listen to pending transactions callback.
func (app *App) RegisterPendingTxListener(listener evmante.PendingTxListener) {
	app.pendingTxListeners = append(app.pendingTxListeners, listener)
}

func (app *App) onPendingTx(hash common.Hash) {
	for _, listener := range app.pendingTxListeners {
		listener(hash)
	}
}

// RegisterSwaggerAPI registers swagger route with API Server
func RegisterSwaggerAPI(_ client.Context, rtr *mux.Router) {
	root, err := fs.Sub(docs.SwaggerUI, "swagger-ui")
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(http.FS(root))
	rtr.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/", staticServer))
}

// GetMaccPerms returns a copy of the module account permissions
func GetMaccPerms() map[string][]string {
	dupMaccPerms := make(map[string][]string)
	for k, v := range maccPerms {
		dupMaccPerms[k] = v
	}
	return dupMaccPerms
}

// initParamsKeeper init params keeper and its subspaces
func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey storetypes.StoreKey) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	// SDK subspaces
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govv1.ParamKeyTable()) //nolint: staticcheck
	paramsKeeper.Subspace(crisistypes.ModuleName)
	// register the key tables for legacy param subspaces
	keyTable := ibcclienttypes.ParamKeyTable()
	keyTable.RegisterParamSet(&ibcconnectiontypes.Params{})
	paramsKeeper.Subspace(ibcexported.ModuleName).WithKeyTable(keyTable)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName).WithKeyTable(ibctransfertypes.ParamKeyTable())
	paramsKeeper.Subspace(icacontrollertypes.SubModuleName).WithKeyTable(icacontrollertypes.ParamKeyTable())
	paramsKeeper.Subspace(evmtypes.ModuleName).WithKeyTable(v0evmtypes.ParamKeyTable()) //nolint: staticcheck
	paramsKeeper.Subspace(feemarkettypes.ModuleName).WithKeyTable(feemarkettypes.ParamKeyTable())
	// this line is used by starport scaffolding # stargate/app/paramSubspace
	paramsKeeper.Subspace(cronostypes.ModuleName).WithKeyTable(cronostypes.ParamKeyTable())

	return paramsKeeper
}

// VerifyAddressFormat verifies whether the address is compatible with Ethereum
func VerifyAddressFormat(bz []byte) error {
	if len(bz) == 0 {
		return errors.Wrap(sdkerrors.ErrUnknownAddress, "invalid address; cannot be empty")
	}
	if len(bz) != AddrLen {
		return errors.Wrapf(
			sdkerrors.ErrUnknownAddress,
			"invalid address length; got: %d, expect: %d", len(bz), AddrLen,
		)
	}

	return nil
}

// Close will be called in graceful shutdown in start cmd
func (app *App) Close() error {
	errs := []error{app.BaseApp.Close()}

	// flush the versiondb
	if closer, ok := app.qms.(io.Closer); ok {
		errs = append(errs, closer.Close())
	}

	// mainly to flush memiavl
	if closer, ok := app.CommitMultiStore().(io.Closer); ok {
		errs = append(errs, closer.Close())
	}

	err := stderrors.Join(errs...)
	msg := "Application gracefully shutdown"
	if err == nil {
		app.Logger().Info(msg)
	} else {
		app.Logger().Error(msg, "error", err)
	}
	return err
}

func maxParallelism() int {
	return min(stdruntime.GOMAXPROCS(0), stdruntime.NumCPU())
}

func (app *App) CheckTx(req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	if app.dummyCheckTx {
		tx, err := app.txDecoder(req.Tx)
		if err != nil {
			return nil, err
		}

		feeTx, ok := tx.(sdk.FeeTx)
		if !ok {
			return nil, errors.Wrap(sdkerrors.ErrInvalidRequest, "tx must be FeeTx")
		}

		return &abci.ResponseCheckTx{Code: abci.CodeTypeOK, GasWanted: int64(feeTx.GetGas())}, nil
	}

	return app.BaseApp.CheckTx(req)
}
