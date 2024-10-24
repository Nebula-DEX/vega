// Copyright (C) 2023 Gobalsky Labs Limited
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package protocol

import (
	"context"
	"fmt"

	"code.vegaprotocol.io/vega/core/activitystreak"
	"code.vegaprotocol.io/vega/core/assets"
	"code.vegaprotocol.io/vega/core/banking"
	"code.vegaprotocol.io/vega/core/blockchain"
	"code.vegaprotocol.io/vega/core/blockchain/abci"
	"code.vegaprotocol.io/vega/core/blockchain/nullchain"
	"code.vegaprotocol.io/vega/core/bridges"
	"code.vegaprotocol.io/vega/core/broker"
	"code.vegaprotocol.io/vega/core/checkpoint"
	ethclient "code.vegaprotocol.io/vega/core/client/eth"
	"code.vegaprotocol.io/vega/core/collateral"
	"code.vegaprotocol.io/vega/core/config"
	"code.vegaprotocol.io/vega/core/datasource"
	"code.vegaprotocol.io/vega/core/datasource/external/ethcall"
	"code.vegaprotocol.io/vega/core/datasource/external/ethverifier"
	"code.vegaprotocol.io/vega/core/datasource/spec"
	"code.vegaprotocol.io/vega/core/datasource/spec/adaptors"
	oracleAdaptors "code.vegaprotocol.io/vega/core/datasource/spec/adaptors"
	"code.vegaprotocol.io/vega/core/delegation"
	"code.vegaprotocol.io/vega/core/epochtime"
	"code.vegaprotocol.io/vega/core/evtforward"
	"code.vegaprotocol.io/vega/core/evtforward/ethereum"
	"code.vegaprotocol.io/vega/core/execution"
	"code.vegaprotocol.io/vega/core/execution/common"
	"code.vegaprotocol.io/vega/core/genesis"
	"code.vegaprotocol.io/vega/core/governance"
	"code.vegaprotocol.io/vega/core/limits"
	"code.vegaprotocol.io/vega/core/netparams"
	"code.vegaprotocol.io/vega/core/netparams/checks"
	"code.vegaprotocol.io/vega/core/netparams/dispatch"
	"code.vegaprotocol.io/vega/core/nodewallets"
	"code.vegaprotocol.io/vega/core/notary"
	"code.vegaprotocol.io/vega/core/parties"
	"code.vegaprotocol.io/vega/core/pow"
	"code.vegaprotocol.io/vega/core/processor"
	"code.vegaprotocol.io/vega/core/protocolupgrade"
	"code.vegaprotocol.io/vega/core/referral"
	"code.vegaprotocol.io/vega/core/rewards"
	"code.vegaprotocol.io/vega/core/snapshot"
	"code.vegaprotocol.io/vega/core/spam"
	"code.vegaprotocol.io/vega/core/staking"
	"code.vegaprotocol.io/vega/core/statevar"
	"code.vegaprotocol.io/vega/core/stats"
	"code.vegaprotocol.io/vega/core/teams"
	"code.vegaprotocol.io/vega/core/txcache"
	"code.vegaprotocol.io/vega/core/types"
	"code.vegaprotocol.io/vega/core/validators"
	"code.vegaprotocol.io/vega/core/validators/erc20multisig"
	"code.vegaprotocol.io/vega/core/vegatime"
	"code.vegaprotocol.io/vega/core/vesting"
	"code.vegaprotocol.io/vega/core/volumediscount"
	"code.vegaprotocol.io/vega/core/volumerebate"
	"code.vegaprotocol.io/vega/libs/subscribers"
	"code.vegaprotocol.io/vega/logging"
	"code.vegaprotocol.io/vega/paths"
	"code.vegaprotocol.io/vega/version"
)

type EthCallEngine interface {
	Start()
	StartAtHeight(height uint64, timestamp uint64)
	Stop()
	MakeResult(specID string, bytes []byte) (ethcall.Result, error)
	CallSpec(ctx context.Context, id string, atBlock uint64) (ethcall.Result, error)
	GetEthTime(ctx context.Context, atBlock uint64) (uint64, error)
	GetRequiredConfirmations(id string) (uint64, error)
	GetInitialTriggerTime(id string) (uint64, error)
	OnSpecActivated(ctx context.Context, spec datasource.Spec) error
	OnSpecDeactivated(ctx context.Context, spec datasource.Spec)
	EnsureChainID(ctx context.Context, chainID string, blockInterval uint64, confirmWithClient bool)
}

type allServices struct {
	ctx             context.Context
	log             *logging.Logger
	confWatcher     *config.Watcher
	confListenerIDs []int
	conf            config.Config

	broker *broker.Broker

	timeService  *vegatime.Svc
	epochService *epochtime.Svc
	eventService *subscribers.Service

	blockchainClient *blockchain.Client

	stats *stats.Stats

	vegaPaths paths.Paths

	marketActivityTracker   *common.MarketActivityTracker
	statevar                *statevar.Engine
	snapshotEngine          *snapshot.Engine
	executionEngine         *execution.Engine
	governance              *governance.Engine
	collateral              *collateral.Engine
	oracle                  *spec.Engine
	oracleAdaptors          *adaptors.Adaptors
	netParams               *netparams.Store
	delegation              *delegation.Engine
	limits                  *limits.Engine
	rewards                 *rewards.Engine
	checkpoint              *checkpoint.Engine
	spam                    *spam.Engine
	pow                     processor.PoWEngine
	builtinOracle           *spec.Builtin
	codec                   abci.Codec
	ethereumOraclesVerifier *ethverifier.Verifier

	partiesEngine *parties.SnapshottedEngine
	txCache       *txcache.TxCache

	assets                *assets.Service
	topology              *validators.Topology
	notary                *notary.SnapshotNotary
	ethCallEngine         EthCallEngine
	witness               *validators.Witness
	banking               *banking.Engine
	genesisHandler        *genesis.Handler
	protocolUpgradeEngine *protocolupgrade.Engine

	teamsEngine     *teams.SnapshottedEngine
	referralProgram *referral.SnapshottedEngine

	primaryEventForwarder       *evtforward.Forwarder
	forwarderHeartbeat          *evtforward.Tracker
	primaryEventForwarderEngine EventForwarderEngine
	primaryEthConfirmations     *ethclient.EthereumConfirmations
	primaryEthClient            *ethclient.PrimaryClient
	primaryBridgeView           *bridges.ERC20LogicView
	primaryMultisig             *erc20multisig.Topology

	secondaryEventForwarderEngine EventForwarderEngine
	secondaryEthConfirmations     *ethclient.EthereumConfirmations
	secondaryEthClient            *ethclient.SecondaryClient
	secondaryBridgeView           *bridges.ERC20LogicView
	secondaryMultisig             *erc20multisig.Topology

	// staking
	stakingAccounts *staking.Accounting
	stakeVerifier   *staking.StakeVerifier
	stakeCheckpoint *staking.Checkpoint

	commander  *nodewallets.Commander
	gastimator *processor.Gastimator

	activityStreak *activitystreak.SnapshotEngine
	vesting        *vesting.SnapshotEngine
	volumeDiscount *volumediscount.SnapshottedEngine
	volumeRebate   *volumerebate.SnapshottedEngine

	// l2 stuff
	// TODO: instantiate
	l2Clients     *ethclient.L2Clients
	l2Verifiers   *ethverifier.L2Verifiers
	l2CallEngines *L2EthCallEngines
}

func newServices(
	ctx context.Context,
	log *logging.Logger,
	conf *config.Watcher,
	nodeWallets *nodewallets.NodeWallets,
	primaryEthClient *ethclient.PrimaryClient,
	secondaryEthClient *ethclient.SecondaryClient,
	primaryEthConfirmations *ethclient.EthereumConfirmations,
	secondaryEthConfirmations *ethclient.EthereumConfirmations,
	blockchainClient *blockchain.Client,
	vegaPaths paths.Paths,
	stats *stats.Stats,
	l2Clients *ethclient.L2Clients,
) (_ *allServices, err error) {
	svcs := &allServices{
		ctx:                       ctx,
		log:                       log,
		confWatcher:               conf,
		conf:                      conf.Get(),
		primaryEthClient:          primaryEthClient,
		secondaryEthClient:        secondaryEthClient,
		l2Clients:                 l2Clients,
		primaryEthConfirmations:   primaryEthConfirmations,
		secondaryEthConfirmations: secondaryEthConfirmations,
		blockchainClient:          blockchainClient,
		stats:                     stats,
		vegaPaths:                 vegaPaths,
	}

	svcs.broker, err = broker.New(svcs.ctx, svcs.log, svcs.conf.Broker, stats.Blockchain)
	if err != nil {
		svcs.log.Error("unable to initialise broker", logging.Error(err))
		return nil, err
	}

	svcs.timeService = vegatime.New(svcs.conf.Time, svcs.broker)
	svcs.epochService = epochtime.NewService(svcs.log, svcs.conf.Epoch, svcs.broker)

	// if we are not a validator, no need to instantiate the commander
	if svcs.conf.IsValidator() {
		// we cannot pass the Chain dependency here (that's set by the blockchain)
		svcs.commander, err = nodewallets.NewCommander(
			svcs.conf.NodeWallet, svcs.log, blockchainClient, nodeWallets.Vega, svcs.stats)
		if err != nil {
			return nil, err
		}
	}

	svcs.genesisHandler = genesis.New(svcs.log, svcs.conf.Genesis)
	svcs.genesisHandler.OnGenesisTimeLoaded(svcs.timeService.SetTimeNow)

	svcs.eventService = subscribers.NewService(svcs.log, svcs.broker, svcs.conf.Broker.EventBusClientBufferSize)
	svcs.collateral = collateral.New(svcs.log, svcs.conf.Collateral, svcs.timeService, svcs.broker)
	svcs.epochService.NotifyOnEpoch(svcs.collateral.OnEpochEvent, svcs.collateral.OnEpochRestore)
	svcs.limits = limits.New(svcs.log, svcs.conf.Limits, svcs.timeService, svcs.broker)

	svcs.netParams = netparams.New(svcs.log, svcs.conf.NetworkParameters, svcs.broker)

	svcs.primaryMultisig = erc20multisig.NewERC20MultisigTopology(svcs.conf.ERC20MultiSig, svcs.log, nil, svcs.broker, svcs.primaryEthClient, svcs.primaryEthConfirmations, svcs.netParams, "primary")
	svcs.secondaryMultisig = erc20multisig.NewERC20MultisigTopology(svcs.conf.ERC20MultiSig, svcs.log, nil, svcs.broker, svcs.secondaryEthClient, svcs.secondaryEthConfirmations, svcs.netParams, "secondary")

	if svcs.conf.IsValidator() {
		svcs.topology = validators.NewTopology(svcs.log, svcs.conf.Validators, validators.WrapNodeWallets(nodeWallets), svcs.broker, svcs.conf.IsValidator(), svcs.commander, svcs.primaryMultisig, svcs.secondaryMultisig, svcs.timeService)
	} else {
		svcs.topology = validators.NewTopology(svcs.log, svcs.conf.Validators, nil, svcs.broker, svcs.conf.IsValidator(), nil, svcs.primaryMultisig, svcs.secondaryMultisig, svcs.timeService)
	}

	svcs.protocolUpgradeEngine = protocolupgrade.New(svcs.log, svcs.conf.ProtocolUpgrade, svcs.broker, svcs.topology, version.Get())
	svcs.witness = validators.NewWitness(svcs.ctx, svcs.log, svcs.conf.Validators, svcs.topology, svcs.commander, svcs.timeService)

	// this is done to go around circular deps...
	svcs.primaryMultisig.SetWitness(svcs.witness)
	svcs.secondaryMultisig.SetWitness(svcs.witness)
	svcs.primaryEventForwarder = evtforward.New(svcs.log, svcs.conf.EvtForward, svcs.commander, svcs.timeService, svcs.topology)
	svcs.forwarderHeartbeat = evtforward.NewTracker(log, svcs.witness, svcs.timeService)

	if svcs.conf.HaveEthClient() {
		if len(svcs.conf.EvtForward.EVMBridges) != 1 {
			return nil, fmt.Errorf("require exactly 1 [[EvtForward.EVMBridges]] in configuration file, got: %d", len(svcs.conf.EvtForward.EVMBridges))
		}

		svcs.primaryBridgeView = bridges.NewERC20LogicView(primaryEthClient, primaryEthConfirmations)
		svcs.secondaryBridgeView = bridges.NewERC20LogicView(secondaryEthClient, secondaryEthConfirmations)
		svcs.primaryEventForwarderEngine = evtforward.NewEngine(svcs.log, svcs.conf.EvtForward.Ethereum)
		svcs.secondaryEventForwarderEngine = evtforward.NewEngine(svcs.log, svcs.conf.EvtForward.EVMBridges[0])
	} else {
		svcs.primaryEventForwarderEngine = evtforward.NewNoopEngine(svcs.log, svcs.conf.EvtForward.Ethereum)
		svcs.secondaryEventForwarderEngine = evtforward.NewNoopEngine(svcs.log, ethereum.NewDefaultConfig())
	}

	svcs.oracle = spec.NewEngine(svcs.log, svcs.conf.Oracles, svcs.timeService, svcs.broker)

	svcs.ethCallEngine = ethcall.NewEngine(svcs.log, svcs.conf.EvtForward.EthCall, svcs.conf.IsValidator(), svcs.primaryEthClient, svcs.primaryEventForwarder)

	svcs.l2CallEngines = NewL2EthCallEngines(svcs.log, svcs.conf.EvtForward.EthCall, svcs.conf.IsValidator(), svcs.l2Clients, svcs.primaryEventForwarder, svcs.oracle.AddSpecActivationListener)

	svcs.ethereumOraclesVerifier = ethverifier.New(svcs.log, svcs.witness, svcs.timeService, svcs.broker,
		svcs.oracle, svcs.ethCallEngine, svcs.primaryEthConfirmations, svcs.conf.HaveEthClient())

	svcs.l2Verifiers = ethverifier.NewL2Verifiers(svcs.log, svcs.witness, svcs.timeService, svcs.broker,
		svcs.oracle, svcs.l2Clients, svcs.l2CallEngines, svcs.conf.IsValidator())

	// Not using the activation event bus event here as on recovery the ethCallEngine needs to have all specs - is this necessary?
	svcs.oracle.AddSpecActivationListener(svcs.ethCallEngine)

	svcs.builtinOracle = spec.NewBuiltin(svcs.oracle, svcs.timeService)
	svcs.oracleAdaptors = oracleAdaptors.New()

	// this is done to go around circular deps again..s
	svcs.primaryMultisig.SetEthereumEventSource(svcs.forwarderHeartbeat)
	svcs.secondaryMultisig.SetEthereumEventSource(svcs.forwarderHeartbeat)

	svcs.stakingAccounts, svcs.stakeVerifier, svcs.stakeCheckpoint = staking.New(
		svcs.log, svcs.conf.Staking, svcs.timeService, svcs.broker, svcs.witness, svcs.primaryEthClient, svcs.netParams, svcs.primaryEventForwarder, svcs.conf.HaveEthClient(), svcs.primaryEthConfirmations, svcs.forwarderHeartbeat,
	)
	svcs.epochService.NotifyOnEpoch(svcs.topology.OnEpochEvent, svcs.topology.OnEpochRestore)
	svcs.epochService.NotifyOnEpoch(stats.OnEpochEvent, stats.OnEpochRestore)

	svcs.teamsEngine = teams.NewSnapshottedEngine(svcs.broker, svcs.timeService)

	svcs.partiesEngine = parties.NewSnapshottedEngine(svcs.broker)
	svcs.txCache = txcache.NewTxCache(svcs.commander)

	svcs.statevar = statevar.New(svcs.log, svcs.conf.StateVar, svcs.broker, svcs.topology, svcs.commander)
	svcs.marketActivityTracker = common.NewMarketActivityTracker(svcs.log, svcs.teamsEngine, svcs.stakingAccounts, svcs.broker, svcs.collateral)

	svcs.notary = notary.NewWithSnapshot(svcs.log, svcs.conf.Notary, svcs.topology, svcs.broker, svcs.commander)

	if svcs.conf.IsValidator() {
		svcs.assets, err = assets.New(ctx, svcs.log, svcs.conf.Assets, nodeWallets.Ethereum, svcs.primaryEthClient, svcs.secondaryEthClient, svcs.broker, svcs.primaryBridgeView, svcs.secondaryBridgeView, svcs.notary, svcs.conf.HaveEthClient())
		if err != nil {
			return nil, fmt.Errorf("could not initialize assets engine: %w", err)
		}
	} else {
		svcs.assets, err = assets.New(ctx, svcs.log, svcs.conf.Assets, nil, nil, nil, svcs.broker, nil, nil, svcs.notary, svcs.conf.HaveEthClient())
		if err != nil {
			return nil, fmt.Errorf("could not initialize assets engine: %w", err)
		}
	}

	// TODO(): this is not pretty
	svcs.topology.SetNotary(svcs.notary)

	// The referral program is used to compute rewards, and can end when reaching
	// the end of epoch. Since the engine will reject computations when the program
	// is marked as ended, it needs to be one of the last service to register on
	// epoch update, so the computation can happen for this epoch.
	svcs.referralProgram = referral.NewSnapshottedEngine(svcs.broker, svcs.timeService, svcs.marketActivityTracker, svcs.stakingAccounts)
	// The referral program engine must be notified of the epoch change *after* the
	// market activity tracker, as it relies on computation that must happen, at
	// the end of the epoch, in market activity tracker.
	svcs.epochService.NotifyOnEpoch(svcs.referralProgram.OnEpoch, svcs.referralProgram.OnEpochRestore)

	svcs.volumeDiscount = volumediscount.NewSnapshottedEngine(svcs.broker, svcs.marketActivityTracker)
	svcs.epochService.NotifyOnEpoch(
		svcs.volumeDiscount.OnEpoch,
		svcs.volumeDiscount.OnEpochRestore,
	)

	svcs.volumeRebate = volumerebate.NewSnapshottedEngine(svcs.broker, svcs.marketActivityTracker)
	svcs.banking = banking.New(svcs.log, svcs.conf.Banking, svcs.collateral, svcs.witness, svcs.timeService,
		svcs.assets, svcs.notary, svcs.broker, svcs.topology, svcs.marketActivityTracker, svcs.primaryBridgeView,
		svcs.secondaryBridgeView, svcs.forwarderHeartbeat, svcs.partiesEngine, svcs.stakingAccounts)

	// instantiate the execution engine
	svcs.executionEngine = execution.NewEngine(
		svcs.log, svcs.conf.Execution, svcs.timeService, svcs.collateral, svcs.oracle, svcs.broker, svcs.statevar,
		svcs.marketActivityTracker, svcs.assets, svcs.referralProgram, svcs.volumeDiscount, svcs.volumeRebate, svcs.banking, svcs.partiesEngine,
		svcs.txCache,
	)
	svcs.epochService.NotifyOnEpoch(svcs.executionEngine.OnEpochEvent, svcs.executionEngine.OnEpochRestore)
	svcs.epochService.NotifyOnEpoch(svcs.marketActivityTracker.OnEpochEvent, svcs.marketActivityTracker.OnEpochRestore)
	svcs.epochService.NotifyOnEpoch(svcs.banking.OnEpoch, svcs.banking.OnEpochRestore)
	svcs.epochService.NotifyOnEpoch(svcs.volumeRebate.OnEpoch, svcs.volumeRebate.OnEpochRestore)

	svcs.gastimator = processor.NewGastimator(svcs.executionEngine)

	svcs.spam = spam.New(svcs.log, svcs.conf.Spam, svcs.epochService, svcs.stakingAccounts)

	if svcs.conf.Blockchain.ChainProvider == blockchain.ProviderNullChain {
		// Use staking-loop to pretend a dummy builtin assets deposited with the faucet was staked
		svcs.codec = &processor.NullBlockchainTxCodec{}

		if svcs.conf.HaveEthClient() {
			svcs.governance = governance.NewEngine(svcs.log, svcs.conf.Governance, svcs.stakingAccounts, svcs.timeService, svcs.broker, svcs.assets, svcs.witness, svcs.executionEngine, svcs.netParams, svcs.banking)
			svcs.delegation = delegation.New(svcs.log, svcs.conf.Delegation, svcs.broker, svcs.topology, svcs.stakingAccounts, svcs.epochService, svcs.timeService)
		} else {
			stakingLoop := nullchain.NewStakingLoop(svcs.collateral, svcs.assets)
			svcs.netParams.Watch([]netparams.WatchParam{
				{
					Param:   netparams.RewardAsset,
					Watcher: stakingLoop.OnStakingAsstUpdate,
				},
			}...)
			svcs.governance = governance.NewEngine(svcs.log, svcs.conf.Governance, stakingLoop, svcs.timeService, svcs.broker, svcs.assets, svcs.witness, svcs.executionEngine, svcs.netParams, svcs.banking)
			svcs.delegation = delegation.New(svcs.log, svcs.conf.Delegation, svcs.broker, svcs.topology, stakingLoop, svcs.epochService, svcs.timeService)
		}

		// disable spam protection based on config
		if !svcs.conf.Blockchain.Null.SpamProtection {
			svcs.spam.DisableSpamProtection() // Disable evaluation for the spam policies by the Spam Engine
		}
	} else {
		svcs.codec = &processor.TxCodec{}
		svcs.governance = governance.NewEngine(svcs.log, svcs.conf.Governance, svcs.stakingAccounts, svcs.timeService, svcs.broker, svcs.assets, svcs.witness, svcs.executionEngine, svcs.netParams, svcs.banking)
		svcs.delegation = delegation.New(svcs.log, svcs.conf.Delegation, svcs.broker, svcs.topology, svcs.stakingAccounts, svcs.epochService, svcs.timeService)
	}

	svcs.activityStreak = activitystreak.NewSnapshotEngine(svcs.log, svcs.executionEngine, svcs.broker)
	svcs.epochService.NotifyOnEpoch(
		svcs.activityStreak.OnEpochEvent,
		svcs.activityStreak.OnEpochRestore,
	)

	svcs.vesting = vesting.NewSnapshotEngine(svcs.log, svcs.collateral, svcs.activityStreak, svcs.broker, svcs.assets, svcs.partiesEngine, svcs.timeService, svcs.stakingAccounts)
	svcs.timeService.NotifyOnTick(svcs.vesting.OnTick)
	svcs.rewards = rewards.New(svcs.log, svcs.conf.Rewards, svcs.broker, svcs.delegation, svcs.epochService, svcs.collateral, svcs.timeService, svcs.marketActivityTracker, svcs.topology, svcs.vesting, svcs.banking, svcs.activityStreak)

	// register this after the rewards engine is created to make sure the on epoch is called in the right order.
	svcs.epochService.NotifyOnEpoch(svcs.vesting.OnEpochEvent, svcs.vesting.OnEpochRestore)

	svcs.registerTimeServiceCallbacks()

	// checkpoint engine
	svcs.checkpoint, err = checkpoint.New(svcs.log, svcs.conf.Checkpoint, svcs.assets, svcs.collateral, svcs.governance, svcs.netParams, svcs.delegation, svcs.epochService, svcs.topology, svcs.banking, svcs.stakeCheckpoint, svcs.primaryMultisig, svcs.marketActivityTracker, svcs.executionEngine)
	if err != nil {
		return nil, err
	}

	// register the callback to startup stuff when checkpoint is loaded
	svcs.checkpoint.RegisterOnCheckpointLoaded(func(_ context.Context) {
		// checkpoint have been loaded
		// which means that genesis has been loaded as well
		// we should be fully ready to start the event sourcing from ethereum
		svcs.vesting.OnCheckpointLoaded()
	})

	svcs.genesisHandler.OnGenesisAppStateLoaded(
		// be sure to keep this in order.
		// the node upon genesis will load all asset first in the node
		// state. This is important to happened first as we will load the
		// asset which will be considered as the governance tokesvcs.
		svcs.UponGenesis,
		// This needs to happen always after, as it defined the network
		// parameters, one of them is  the Governance Token asset ID.
		// which if not loaded in the previous state, then will make the node
		// panic at startup.
		svcs.netParams.UponGenesis,
		svcs.topology.LoadValidatorsOnGenesis,
		svcs.limits.UponGenesis,
		svcs.checkpoint.UponGenesis,
	)

	svcs.snapshotEngine, err = snapshot.NewEngine(svcs.vegaPaths, svcs.conf.Snapshot, svcs.log, svcs.timeService, svcs.stats.Blockchain)
	if err != nil {
		return nil, fmt.Errorf("could not initialize the snapshot engine: %w", err)
	}

	// notify delegation, rewards, and accounting on changes in the validator pub key
	svcs.topology.NotifyOnKeyChange(svcs.governance.ValidatorKeyChanged)

	svcs.snapshotEngine.AddProviders(
		svcs.txCache,
		svcs.checkpoint,
		svcs.collateral,
		svcs.governance,
		svcs.delegation,
		svcs.netParams,
		svcs.epochService,
		svcs.assets,
		svcs.banking,
		svcs.witness,
		svcs.notary,
		svcs.stakingAccounts,
		svcs.stakeVerifier,
		svcs.limits,
		svcs.topology,
		svcs.primaryEventForwarder,
		svcs.executionEngine,
		svcs.marketActivityTracker,
		svcs.statevar,
		svcs.primaryMultisig,
		erc20multisig.NewEVMTopologies(svcs.secondaryMultisig),
		svcs.protocolUpgradeEngine,
		svcs.ethereumOraclesVerifier,
		svcs.vesting,
		svcs.activityStreak,
		svcs.referralProgram,
		svcs.volumeDiscount,
		svcs.teamsEngine,
		svcs.spam,
		svcs.l2Verifiers,
		svcs.partiesEngine,
		svcs.forwarderHeartbeat,
		svcs.volumeRebate,
	)

	pow := pow.New(svcs.log, svcs.conf.PoW)

	if svcs.conf.Blockchain.ChainProvider == blockchain.ProviderNullChain {
		pow.DisableVerification()
	}
	svcs.pow = pow
	svcs.snapshotEngine.AddProviders(pow)
	powWatchers := []netparams.WatchParam{
		{
			Param:   netparams.SpamPoWNumberOfPastBlocks,
			Watcher: pow.UpdateSpamPoWNumberOfPastBlocks,
		},
		{
			Param:   netparams.SpamPoWDifficulty,
			Watcher: pow.UpdateSpamPoWDifficulty,
		},
		{
			Param:   netparams.SpamPoWHashFunction,
			Watcher: pow.UpdateSpamPoWHashFunction,
		},
		{
			Param:   netparams.SpamPoWIncreasingDifficulty,
			Watcher: pow.UpdateSpamPoWIncreasingDifficulty,
		},
		{
			Param:   netparams.SpamPoWNumberOfTxPerBlock,
			Watcher: pow.UpdateSpamPoWNumberOfTxPerBlock,
		},
	}

	// The team engine is used to know the team a party belongs to. The computation
	// of the referral program rewards requires this information. Since the team
	// switches happen when the end of epoch is reached, it needs to be one of the
	// last services to register on epoch update, so the computation is made based
	// on the team the parties belonged to during the epoch and not the new one.
	svcs.epochService.NotifyOnEpoch(svcs.teamsEngine.OnEpoch, svcs.teamsEngine.OnEpochRestore)

	// setup config reloads for all engines / services /etc
	svcs.registerConfigWatchers()

	// setup some network parameters runtime validations and network parameters
	// updates dispatches this must come before we try to load from a snapshot,
	// which happens in startBlockchain
	if err := svcs.setupNetParameters(powWatchers); err != nil {
		return nil, err
	}

	return svcs, nil
}

func (svcs *allServices) registerTimeServiceCallbacks() {
	svcs.timeService.NotifyOnTick(
		svcs.broker.OnTick,
		svcs.epochService.OnTick,
		svcs.builtinOracle.OnTick,
		svcs.netParams.OnTick,
		svcs.primaryMultisig.OnTick,
		svcs.secondaryMultisig.OnTick,
		svcs.witness.OnTick,

		svcs.primaryEventForwarder.OnTick,
		svcs.stakeVerifier.OnTick,
		svcs.statevar.OnTick,
		svcs.executionEngine.OnTick,
		svcs.delegation.OnTick,
		svcs.notary.OnTick,
		svcs.banking.OnTick,
		svcs.assets.OnTick,
		svcs.limits.OnTick,

		svcs.ethereumOraclesVerifier.OnTick,
		svcs.l2Verifiers.OnTick,
		svcs.forwarderHeartbeat.OnTick,
	)
}

func (svcs *allServices) Stop() {
	svcs.confWatcher.Unregister(svcs.confListenerIDs)
	svcs.primaryEventForwarderEngine.Stop()
	svcs.secondaryEventForwarderEngine.Stop()
	svcs.snapshotEngine.Close()
	svcs.ethCallEngine.Stop()
}

func (svcs *allServices) registerConfigWatchers() {
	svcs.confListenerIDs = svcs.confWatcher.OnConfigUpdateWithID(
		func(cfg config.Config) { svcs.executionEngine.ReloadConf(cfg.Execution) },
		func(cfg config.Config) { svcs.notary.ReloadConf(cfg.Notary) },
		func(cfg config.Config) { svcs.primaryEventForwarderEngine.ReloadConf(cfg.EvtForward.Ethereum) },
		func(cfg config.Config) { svcs.primaryEventForwarder.ReloadConf(cfg.EvtForward) },
		func(cfg config.Config) {
			if len(cfg.EvtForward.EVMBridges) > 0 {
				svcs.secondaryEventForwarderEngine.ReloadConf(cfg.EvtForward.EVMBridges[0])
			}
		},
		func(cfg config.Config) { svcs.topology.ReloadConf(cfg.Validators) },
		func(cfg config.Config) { svcs.witness.ReloadConf(cfg.Validators) },
		func(cfg config.Config) { svcs.assets.ReloadConf(cfg.Assets) },
		func(cfg config.Config) { svcs.banking.ReloadConf(cfg.Banking) },
		func(cfg config.Config) { svcs.governance.ReloadConf(cfg.Governance) },
		func(cfg config.Config) { svcs.stats.ReloadConf(cfg.Stats) },
		func(cfg config.Config) { svcs.broker.ReloadConf(cfg.Broker) },
	)

	if svcs.conf.HaveEthClient() {
		svcs.confListenerIDs = svcs.confWatcher.OnConfigUpdateWithID(
			func(cfg config.Config) { svcs.l2Clients.ReloadConf(cfg.Ethereum) },
		)
	}

	svcs.timeService.NotifyOnTick(svcs.confWatcher.OnTimeUpdate)
}

func (svcs *allServices) setupNetParameters(powWatchers []netparams.WatchParam) error {
	// now we are going to setup some network parameters which can be done
	// through runtime checks
	// e.g: changing the governance asset require the Assets and Collateral engines, so we can ensure any changes there are made for a valid asset

	if err := svcs.netParams.AddRules(
		netparams.ParamStringRules(
			netparams.RewardAsset,
			checks.RewardAssetUpdate(svcs.log, svcs.assets, svcs.collateral),
		),
	); err != nil {
		return err
	}

	spamWatchers := []netparams.WatchParam{}
	if svcs.spam != nil {
		spamWatchers = []netparams.WatchParam{
			{
				Param:   netparams.MarketAggressiveOrderBlockDelay,
				Watcher: svcs.txCache.OnNumBlocksToDelayUpdated,
			},
			{
				Param:   netparams.SpamProtectionMaxVotes,
				Watcher: svcs.spam.OnMaxVotesChanged,
			},
			{
				Param:   netparams.StakingAndDelegationRewardMinimumValidatorStake,
				Watcher: svcs.spam.OnMinValidatorTokensChanged,
			},
			{
				Param:   netparams.SpamProtectionMaxProposals,
				Watcher: svcs.spam.OnMaxProposalsChanged,
			},
			{
				Param:   netparams.SpamProtectionMaxDelegations,
				Watcher: svcs.spam.OnMaxDelegationsChanged,
			},
			{
				Param:   netparams.SpamProtectionMinTokensForProposal,
				Watcher: svcs.spam.OnMinTokensForProposalChanged,
			},
			{
				Param:   netparams.SpamProtectionMinTokensForVoting,
				Watcher: svcs.spam.OnMinTokensForVotingChanged,
			},
			{
				Param:   netparams.SpamProtectionMinTokensForDelegation,
				Watcher: svcs.spam.OnMinTokensForDelegationChanged,
			},
			{
				Param:   netparams.TransferMaxCommandsPerEpoch,
				Watcher: svcs.spam.OnMaxTransfersChanged,
			},
			{
				Param:   netparams.SpamProtectionMinMultisigUpdates,
				Watcher: svcs.spam.OnMinTokensForMultisigUpdatesChanged,
			},
			{
				Param:   netparams.SpamProtectionMaxMultisigUpdates,
				Watcher: svcs.spam.OnMaxMultisigUpdatesChanged,
			},
			{
				Param:   netparams.ReferralProgramMinStakedVegaTokens,
				Watcher: svcs.spam.OnMinTokensForReferral,
			},
			{
				Param:   netparams.SpamProtectionMaxCreateReferralSet,
				Watcher: svcs.spam.OnMaxCreateReferralSet,
			},
			{
				Param:   netparams.SpamProtectionMaxUpdatePartyProfile,
				Watcher: svcs.spam.OnMaxPartyProfile,
			},
			{
				Param:   netparams.SpamProtectionMaxUpdateReferralSet,
				Watcher: svcs.spam.OnMaxUpdateReferralSet,
			},
			{
				Param:   netparams.SpamProtectionMaxApplyReferralCode,
				Watcher: svcs.spam.OnMaxApplyReferralCode,
			},
		}
	}

	watchers := []netparams.WatchParam{
		{
			Param:   netparams.RewardAsset,
			Watcher: svcs.banking.OnStakingAsset,
		},
		{
			Param:   netparams.RewardAsset,
			Watcher: svcs.vesting.OnStakingAssetUpdate,
		},
		{
			Param:   netparams.SpamProtectionBalanceSnapshotFrequency,
			Watcher: svcs.collateral.OnBalanceSnapshotFrequencyUpdated,
		},
		{
			Param:   netparams.MinEpochsInTeamForMetricRewardEligibility,
			Watcher: svcs.marketActivityTracker.OnMinEpochsInTeamForRewardEligibilityUpdated,
		},
		{
			Param:   netparams.MinBlockCapacity,
			Watcher: svcs.gastimator.OnMinBlockCapacityUpdate,
		},
		{
			Param:   netparams.MaxGasPerBlock,
			Watcher: svcs.gastimator.OnMaxGasUpdate,
		},
		{
			Param:   netparams.DefaultGas,
			Watcher: svcs.gastimator.OnDefaultGasUpdate,
		},
		{
			Param:   netparams.ValidatorsVoteRequired,
			Watcher: svcs.protocolUpgradeEngine.OnRequiredMajorityChanged,
		},
		{
			Param:   netparams.ValidatorPerformanceScalingFactor,
			Watcher: svcs.topology.OnPerformanceScalingChanged,
		},
		{
			Param:   netparams.ValidatorsEpochLength,
			Watcher: svcs.topology.OnEpochLengthUpdate,
		},
		{
			Param:   netparams.NumberOfTendermintValidators,
			Watcher: svcs.topology.UpdateNumberOfTendermintValidators,
		},
		{
			Param:   netparams.ValidatorIncumbentBonus,
			Watcher: svcs.topology.UpdateValidatorIncumbentBonusFactor,
		},
		{
			Param:   netparams.NumberEthMultisigSigners,
			Watcher: svcs.topology.UpdateNumberEthMultisigSigners,
		},
		{
			Param:   netparams.MultipleOfTendermintValidatorsForEtsatzSet,
			Watcher: svcs.topology.UpdateErsatzValidatorsFactor,
		},
		{
			Param:   netparams.MinimumEthereumEventsForNewValidator,
			Watcher: svcs.topology.UpdateMinimumEthereumEventsForNewValidator,
		},
		{
			Param:   netparams.StakingAndDelegationRewardMinimumValidatorStake,
			Watcher: svcs.topology.UpdateMinimumRequireSelfStake,
		},
		{
			Param:   netparams.DelegationMinAmount,
			Watcher: svcs.delegation.OnMinAmountChanged,
		},
		{
			Param:   netparams.RewardAsset,
			Watcher: dispatch.RewardAssetUpdate(svcs.log, svcs.assets),
		},
		{
			Param:   netparams.MinimumMarginQuantumMultiple,
			Watcher: svcs.executionEngine.OnMinimalMarginQuantumMultipleUpdate,
		},
		{
			Param:   netparams.MinimumHoldingQuantumMultiple,
			Watcher: svcs.executionEngine.OnMinimalHoldingQuantumMultipleUpdate,
		},
		{
			Param:   netparams.MarketMarginScalingFactors,
			Watcher: svcs.executionEngine.OnMarketMarginScalingFactorsUpdate,
		},
		{
			Param:   netparams.MarketFeeFactorsMakerFee,
			Watcher: svcs.executionEngine.OnMarketFeeFactorsMakerFeeUpdate,
		},
		{
			Param:   netparams.MarketFeeFactorsInfrastructureFee,
			Watcher: svcs.executionEngine.OnMarketFeeFactorsInfrastructureFeeUpdate,
		},
		{
			Param:   netparams.MarketFeeFactorsTreasuryFee,
			Watcher: svcs.executionEngine.OnMarketFeeFactorsTreasuryFeeUpdate,
		},
		{
			Param:   netparams.MarketFeeFactorsBuyBackFee,
			Watcher: svcs.executionEngine.OnMarketFeeFactorsBuyBackFeeUpdate,
		},
		{
			Param:   netparams.MarketFeeFactorsTreasuryFee,
			Watcher: svcs.volumeRebate.OnMarketFeeFactorsTreasuryFeeUpdate,
		},
		{
			Param:   netparams.MarketFeeFactorsBuyBackFee,
			Watcher: svcs.volumeRebate.OnMarketFeeFactorsBuyBackFeeUpdate,
		},
		{
			Param:   netparams.MarketValueWindowLength,
			Watcher: svcs.executionEngine.OnMarketValueWindowLengthUpdate,
		},
		{
			Param:   netparams.NetworkWideAuctionDuration,
			Watcher: svcs.executionEngine.OnNetworkWideAuctionDurationUpdated,
		},
		{
			Param: netparams.BlockchainsPrimaryEthereumConfig,
			Watcher: func(ctx context.Context, cfg interface{}) error {
				ethCfg, err := types.EthereumConfigFromUntypedProto(cfg)
				if err != nil {
					return fmt.Errorf("invalid primary ethereum configuration: %w", err)
				}

				if err := svcs.primaryEthClient.UpdateEthereumConfig(ctx, ethCfg); err != nil {
					return err
				}

				svcs.assets.SetBridgeChainID(ethCfg.ChainID(), true)
				if err := svcs.primaryEventForwarderEngine.SetupEthereumEngine(
					svcs.primaryEthClient,
					svcs.primaryEventForwarder,
					svcs.conf.EvtForward.Ethereum,
					ethCfg,
					svcs.assets); err != nil {
					return err
				}
				svcs.forwarderHeartbeat.RegisterForwarder(
					svcs.primaryEventForwarderEngine,
					ethCfg.ChainID(),
					ethCfg.CollateralBridge().HexAddress(),
					ethCfg.StakingBridge().HexAddress(),
					ethCfg.VestingBridge().HexAddress(),
					ethCfg.MultiSigControl().HexAddress(),
				)
				return nil
			},
		},
		{
			Param: netparams.BlockchainsEVMBridgeConfigs,
			Watcher: func(ctx context.Context, cfg interface{}) error {
				cfgs, err := types.EVMChainConfigFromUntypedProto(cfg)
				if err != nil {
					return fmt.Errorf("invalid secondary ethereum configuration: %w", err)
				}

				ethCfg := cfgs.Configs[0]

				if err := svcs.secondaryEthClient.UpdateEthereumConfig(ctx, ethCfg); err != nil {
					return err
				}

				svcs.assets.SetBridgeChainID(ethCfg.ChainID(), false)

				var bridgeCfg ethereum.Config
				if svcs.conf.HaveEthClient() {
					bridgeCfg = svcs.conf.EvtForward.EVMBridges[0]
				}

				if err := svcs.secondaryEventForwarderEngine.SetupSecondaryEthereumEngine(
					svcs.secondaryEthClient,
					svcs.primaryEventForwarder,
					bridgeCfg,
					ethCfg,
					svcs.assets,
				); err != nil {
					return err
				}

				svcs.forwarderHeartbeat.RegisterForwarder(
					svcs.secondaryEventForwarderEngine,
					ethCfg.ChainID(),
					ethCfg.CollateralBridge().HexAddress(),
					ethCfg.MultiSigControl().HexAddress(),
				)

				return nil
			},
		},
		{
			Param:   netparams.MaxPeggedOrders,
			Watcher: svcs.executionEngine.OnMaxPeggedOrderUpdate,
		},
		{
			Param:   netparams.MarketMinLpStakeQuantumMultiple,
			Watcher: svcs.executionEngine.OnMinLpStakeQuantumMultipleUpdate,
		},
		{
			Param:   netparams.RewardMarketCreationQuantumMultiple,
			Watcher: svcs.executionEngine.OnMarketCreationQuantumMultipleUpdate,
		},
		{
			Param:   netparams.MarketLiquidityMaximumLiquidityFeeFactorLevel,
			Watcher: svcs.executionEngine.OnMarketLiquidityMaximumLiquidityFeeFactorLevelUpdate,
		},
		{
			Param:   netparams.MarketAuctionMinimumDuration,
			Watcher: svcs.executionEngine.OnMarketAuctionMinimumDurationUpdate,
		},
		{
			Param:   netparams.MarketAuctionMaximumDuration,
			Watcher: svcs.executionEngine.OnMarketAuctionMaximumDurationUpdate,
		},
		{
			Param:   netparams.MarketProbabilityOfTradingTauScaling,
			Watcher: svcs.executionEngine.OnMarketProbabilityOfTradingTauScalingUpdate,
		},
		{
			Param:   netparams.MarketMinProbabilityOfTradingForLPOrders,
			Watcher: svcs.executionEngine.OnMarketMinProbabilityOfTradingForLPOrdersUpdate,
		},
		// Liquidity version 2.
		{
			Param:   netparams.MarketLiquidityBondPenaltyParameter,
			Watcher: svcs.executionEngine.OnMarketLiquidityV2BondPenaltyUpdate,
		},
		{
			Param:   netparams.MarketLiquidityEarlyExitPenalty,
			Watcher: svcs.executionEngine.OnMarketLiquidityV2EarlyExitPenaltyUpdate,
		},
		{
			Param:   netparams.MarketLiquidityMaximumLiquidityFeeFactorLevel,
			Watcher: svcs.executionEngine.OnMarketLiquidityV2MaximumLiquidityFeeFactorLevelUpdate,
		},
		{
			Param:   netparams.MarketLiquiditySLANonPerformanceBondPenaltySlope,
			Watcher: svcs.executionEngine.OnMarketLiquidityV2SLANonPerformanceBondPenaltySlopeUpdate,
		},
		{
			Param:   netparams.MarketLiquiditySLANonPerformanceBondPenaltyMax,
			Watcher: svcs.executionEngine.OnMarketLiquidityV2SLANonPerformanceBondPenaltyMaxUpdate,
		},
		{
			Param:   netparams.MarketLiquidityStakeToCCYVolume,
			Watcher: svcs.executionEngine.OnMarketLiquidityV2StakeToCCYVolumeUpdate,
		},
		{
			Param:   netparams.MarketLiquidityProvidersFeeCalculationTimeStep,
			Watcher: svcs.executionEngine.OnMarketLiquidityV2ProvidersFeeCalculationTimeStep,
		},
		// End of liquidity version 2.
		{
			Param:   netparams.ValidatorsEpochLength,
			Watcher: svcs.epochService.OnEpochLengthUpdate,
		},
		{
			Param:   netparams.StakingAndDelegationRewardMaxPayoutPerParticipant,
			Watcher: svcs.rewards.UpdateMaxPayoutPerParticipantForStakingRewardScheme,
		},
		{
			Param:   netparams.StakingAndDelegationRewardDelegatorShare,
			Watcher: svcs.rewards.UpdateDelegatorShareForStakingRewardScheme,
		},
		{
			Param:   netparams.StakingAndDelegationRewardMinimumValidatorStake,
			Watcher: svcs.rewards.UpdateMinimumValidatorStakeForStakingRewardScheme,
		},
		{
			Param:   netparams.RewardAsset,
			Watcher: svcs.rewards.UpdateAssetForStakingAndDelegation,
		},
		{
			Param:   netparams.StakingAndDelegationRewardCompetitionLevel,
			Watcher: svcs.rewards.UpdateCompetitionLevelForStakingRewardScheme,
		},
		{
			Param:   netparams.StakingAndDelegationRewardsMinValidators,
			Watcher: svcs.rewards.UpdateMinValidatorsStakingRewardScheme,
		},
		{
			Param:   netparams.StakingAndDelegationRewardOptimalStakeMultiplier,
			Watcher: svcs.rewards.UpdateOptimalStakeMultiplierStakingRewardScheme,
		},
		{
			Param:   netparams.ErsatzvalidatorsRewardFactor,
			Watcher: svcs.rewards.UpdateErsatzRewardFactor,
		},
		{
			Param:   netparams.ValidatorsVoteRequired,
			Watcher: svcs.witness.OnDefaultValidatorsVoteRequiredUpdate,
		},
		{
			Param:   netparams.ValidatorsVoteRequired,
			Watcher: svcs.notary.OnDefaultValidatorsVoteRequiredUpdate,
		},
		{
			Param:   netparams.NetworkCheckpointTimeElapsedBetweenCheckpoints,
			Watcher: svcs.checkpoint.OnTimeElapsedUpdate,
		},
		{
			Param:   netparams.SnapshotIntervalLength,
			Watcher: svcs.snapshotEngine.OnSnapshotIntervalUpdate,
		},
		{
			Param:   netparams.ValidatorsVoteRequired,
			Watcher: svcs.statevar.OnDefaultValidatorsVoteRequiredUpdate,
		},
		{
			Param:   netparams.FloatingPointUpdatesDuration,
			Watcher: svcs.statevar.OnFloatingPointUpdatesDurationUpdate,
		},
		{
			Param:   netparams.TransferFeeFactor,
			Watcher: svcs.banking.OnTransferFeeFactorUpdate,
		},
		{
			Param:   netparams.RewardsUpdateFrequency,
			Watcher: svcs.banking.OnRewardsUpdateFrequencyUpdate,
		},
		{
			Param:   netparams.TransferFeeMaxQuantumAmount,
			Watcher: svcs.banking.OnMaxQuantumAmountUpdate,
		},
		{
			Param:   netparams.TransferFeeDiscountDecayFraction,
			Watcher: svcs.banking.OnTransferFeeDiscountDecayFractionUpdate,
		},
		{
			Param:   netparams.TransferFeeDiscountMinimumTrackedAmount,
			Watcher: svcs.banking.OnTransferFeeDiscountMinimumTrackedAmountUpdate,
		},
		{
			Param:   netparams.GovernanceTransferMaxFraction,
			Watcher: svcs.banking.OnMaxFractionChanged,
		},
		{
			Param:   netparams.GovernanceTransferMaxAmount,
			Watcher: svcs.banking.OnMaxAmountChanged,
		},
		{
			Param:   netparams.TransferMinTransferQuantumMultiple,
			Watcher: svcs.banking.OnMinTransferQuantumMultiple,
		},
		{
			Param:   netparams.SpamProtectionMinimumWithdrawalQuantumMultiple,
			Watcher: svcs.banking.OnMinWithdrawQuantumMultiple,
		},
		{
			Param: netparams.BlockchainsPrimaryEthereumConfig,
			Watcher: func(_ context.Context, cfg interface{}) error {
				// nothing to do if not a validator
				if !svcs.conf.HaveEthClient() {
					return nil
				}
				ethCfg, err := types.EthereumConfigFromUntypedProto(cfg)
				if err != nil {
					return fmt.Errorf("invalid primary ethereum configuration: %w", err)
				}

				svcs.primaryEthConfirmations.UpdateConfirmations(ethCfg.Confirmations())
				return nil
			},
		},
		{
			Param: netparams.BlockchainsEVMBridgeConfigs,
			Watcher: func(_ context.Context, cfg interface{}) error {
				// nothing to do if not a validator
				if !svcs.conf.HaveEthClient() {
					return nil
				}
				ethCfg, err := types.EVMChainConfigFromUntypedProto(cfg)
				if err != nil {
					return fmt.Errorf("invalid secondary ethereum configuration: %w", err)
				}

				svcs.secondaryEthConfirmations.UpdateConfirmations(ethCfg.Configs[0].Confirmations())
				return nil
			},
		},
		{
			Param: netparams.BlockchainsPrimaryEthereumConfig,
			Watcher: func(ctx context.Context, cfg interface{}) error {
				ethCfg, err := types.EthereumConfigFromUntypedProto(cfg)
				if err != nil {
					return fmt.Errorf("invalid primary ethereum configuration: %w", err)
				}

				// every 1 block for the main ethereum chain is acceptable
				svcs.ethCallEngine.EnsureChainID(ctx, ethCfg.ChainID(), 1, svcs.conf.HaveEthClient())

				// nothing to do if not a validator
				if !svcs.conf.HaveEthClient() {
					return nil
				}

				svcs.witness.SetPrimaryDefaultConfirmations(ethCfg.ChainID(), ethCfg.Confirmations())
				return nil
			},
		},
		{
			Param: netparams.BlockchainsPrimaryEthereumConfig,
			Watcher: func(_ context.Context, cfg interface{}) error {
				ethCfg, err := types.EthereumConfigFromUntypedProto(cfg)
				if err != nil {
					return fmt.Errorf("invalid primary ethereum configuration: %w", err)
				}

				svcs.banking.OnPrimaryEthChainIDUpdated(ethCfg.ChainID(), ethCfg.CollateralBridge().HexAddress())
				return nil
			},
		},
		{
			Param: netparams.BlockchainsEVMBridgeConfigs,
			Watcher: func(_ context.Context, cfgs interface{}) error {
				ethCfgs, err := types.EVMChainConfigFromUntypedProto(cfgs)
				if err != nil {
					return fmt.Errorf("invalid secondary ethereum configuration: %w", err)
				}

				ethCfg := ethCfgs.Configs[0]
				svcs.banking.OnSecondaryEthChainIDUpdated(ethCfg.ChainID(), ethCfg.CollateralBridge().HexAddress())
				svcs.witness.SetSecondaryDefaultConfirmations(ethCfg.ChainID(), ethCfg.Confirmations(), ethCfg.BlockTime())
				return nil
			},
		},
		{
			Param: netparams.BlockchainsEthereumL2Configs,
			Watcher: func(ctx context.Context, cfg interface{}) error {
				ethCfg, err := types.EthereumL2ConfigsFromUntypedProto(cfg)
				if err != nil {
					return fmt.Errorf("invalid ethereum l2 configuration: %w", err)
				}

				if svcs.conf.HaveEthClient() {
					svcs.l2Clients.UpdateConfirmations(ethCfg)
				}

				// non-validators still need to create these engine's for consensus reasons
				svcs.l2CallEngines.OnEthereumL2ConfigsUpdated(ctx, ethCfg)
				svcs.l2Verifiers.OnEthereumL2ConfigsUpdated(ctx, ethCfg)

				return nil
			},
		},
		{
			Param:   netparams.LimitsProposeMarketEnabledFrom,
			Watcher: svcs.limits.OnLimitsProposeMarketEnabledFromUpdate,
		},
		{
			Param:   netparams.SpotMarketTradingEnabled,
			Watcher: svcs.limits.OnLimitsProposeSpotMarketEnabledFromUpdate,
		},
		{
			Param:   netparams.PerpsMarketTradingEnabled,
			Watcher: svcs.limits.OnLimitsProposePerpsMarketEnabledFromUpdate,
		},
		{
			Param:   netparams.AMMMarketTradingEnabled,
			Watcher: svcs.limits.OnLimitsProposeAMMEnabledUpdate,
		},
		{
			Param:   netparams.LimitsProposeAssetEnabledFrom,
			Watcher: svcs.limits.OnLimitsProposeAssetEnabledFromUpdate,
		},
		{
			Param:   netparams.MarkPriceUpdateMaximumFrequency,
			Watcher: svcs.executionEngine.OnMarkPriceUpdateMaximumFrequency,
		},
		{
			Param:   netparams.InternalCompositePriceUpdateFrequency,
			Watcher: svcs.executionEngine.OnInternalCompositePriceUpdateFrequency,
		},
		{
			Param:   netparams.MarketSuccessorLaunchWindow,
			Watcher: svcs.executionEngine.OnSuccessorMarketTimeWindowUpdate,
		},
		{
			Param:   netparams.SpamProtectionMaxStopOrdersPerMarket,
			Watcher: svcs.executionEngine.OnMarketPartiesMaximumStopOrdersUpdate,
		},
		{
			Param:   netparams.RewardsVestingMinimumTransfer,
			Watcher: svcs.vesting.OnRewardVestingMinimumTransferUpdate,
		},
		{
			Param:   netparams.RewardsVestingBaseRate,
			Watcher: svcs.vesting.OnRewardVestingBaseRateUpdate,
		},
		{
			Param:   netparams.RewardsVestingBenefitTiers,
			Watcher: svcs.vesting.OnBenefitTiersUpdate,
		},
		{
			Param:   netparams.ReferralProgramMaxPartyNotionalVolumeByQuantumPerEpoch,
			Watcher: svcs.referralProgram.OnReferralProgramMaxPartyNotionalVolumeByQuantumPerEpochUpdate,
		},
		{
			Param:   netparams.ReferralProgramMaxReferralRewardProportion,
			Watcher: svcs.referralProgram.OnReferralProgramMaxReferralRewardProportionUpdate,
		},
		{
			Param:   netparams.ReferralProgramMinStakedVegaTokens,
			Watcher: svcs.referralProgram.OnReferralProgramMinStakedVegaTokensUpdate,
		},
		{
			Param:   netparams.ReferralProgramMinStakedVegaTokens,
			Watcher: svcs.teamsEngine.OnReferralProgramMinStakedVegaTokensUpdate,
		},
		{
			Param:   netparams.SpamProtectionApplyReferralMinFunds,
			Watcher: svcs.referralProgram.OnMinBalanceForApplyReferralCodeUpdated,
		},
		{
			Param:   netparams.SpamProtectionReferralSetMinFunds,
			Watcher: svcs.referralProgram.OnMinBalanceForReferralProgramUpdated,
		},
		{
			Param:   netparams.SpamProtectionUpdateProfileMinFunds,
			Watcher: svcs.partiesEngine.OnMinBalanceForUpdatePartyProfileUpdated,
		},
		{
			Param:   netparams.RewardsActivityStreakBenefitTiers,
			Watcher: svcs.activityStreak.OnBenefitTiersUpdate,
		},
		{
			Param:   netparams.RewardsActivityStreakMinQuantumOpenVolume,
			Watcher: svcs.activityStreak.OnMinQuantumOpenNationalVolumeUpdate,
		},
		{
			Param:   netparams.RewardsActivityStreakMinQuantumTradeVolume,
			Watcher: svcs.activityStreak.OnMinQuantumTradeVolumeUpdate,
		},
		{
			Param:   netparams.RewardsActivityStreakInactivityLimit,
			Watcher: svcs.activityStreak.OnRewardsActivityStreakInactivityLimit,
		},
		{
			Param:   netparams.MarketAMMMinCommitmentQuantum,
			Watcher: svcs.executionEngine.OnMarketAMMMinCommitmentQuantum,
		},
		{
			Param:   netparams.MarketAMMMaxCalculationLevels,
			Watcher: svcs.executionEngine.OnMarketAMMMaxCalculationLevels,
		},
		{
			Param:   netparams.MarketLiquidityEquityLikeShareFeeFraction,
			Watcher: svcs.executionEngine.OnMarketLiquidityEquityLikeShareFeeFractionUpdate,
		},
	}

	watchers = append(watchers, powWatchers...)
	watchers = append(watchers, spamWatchers...)

	// now add some watcher for our netparams
	return svcs.netParams.Watch(watchers...)
}
