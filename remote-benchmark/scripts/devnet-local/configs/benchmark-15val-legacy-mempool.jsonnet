// Legacy-mempool variant of benchmark-15val.jsonnet for cronosd binaries that
// predate v1.8.0-alpha's app-mempool feature. Keep in sync with
// benchmark-15val.jsonnet's non-mempool fields by hand - there is no shared
// base to inherit from.
//
// This one is NOT in sync, and deliberately so: it runs goleveldb instead of
// rocksdb, memiavl disabled, and half the block gas (105000000 vs 210000000),
// because the v1.7.8 release binary this config targets cannot be driven at the
// modern config's settings. A 15-validator legacy-vs-modern TPS delta therefore
// measures all four differences at once, not the mempool alone - it is not a
// controlled mempool comparison.
//
// Topology (from the 15-val P2P plan): libp2p disabled here, so
// persistent_peers IS the whole topology on this binary (unlike the modern
// config, where p2p.libp2p.enabled=true makes persistent_peers dead config -
// pystarport's classic switch/PEX are never even constructed there).
// persistent_peers_max_dial_period matters more at 15 nodes than at 5: more
// concurrent first-dial races at startup, so the same 3s cap is kept.
local num_validators = 15;

local validator(i) = {
  coins: '1000000000000000000stake,10000000000000000000000basetcro',
  staked: '1000000000000000000stake',
  mnemonic: '${VALIDATOR%d_MNEMONIC}' % (i + 1),
  client_config: {
    'broadcast-mode': 'sync',
  },
  config: {
    // Same reasoning as benchmark-15val.jsonnet: every node's shared
    // instrumentation.prometheus_listen_addr collides on the modern config's
    // 26650-26699 port range if not given a distinct port per node.
    instrumentation: { prometheus_listen_addr: ':%d' % (26650 + i * 10 + 9) },
  },
};

{
  dotenv: '../../../../scripts/.env',
  'cronos_777-1': {
    cmd: 'cronosd',
    'start-flags': '--trace',
    config: {
      db_backend: 'goleveldb',
      event_bus_buffer_capacity: 128,
      rpc: {
        'max_open_connections': 10000,
      },
      mempool: {
        size: 10000,
        recheck: true,
      },
      consensus: {
        timeout_propose: '3s',
        timeout_propose_delta: '500ms',
        timeout_prevote: '1s',
        timeout_prevote_delta: '500ms',
        timeout_precommit: '1s',
        timeout_precommit_delta: '500ms',
        timeout_commit: '200ms',
        create_empty_blocks_interval: '5s',
        peer_gossip_sleep_duration: '100ms',
        peer_query_maj23_sleep_duration: '2s',
        skip_timeout_commit: false,
      },
      tx_index: {
        indexer: 'null',
      },
      instrumentation: {
        prometheus: true,
        prometheus_listen_addr: ':9090',
      },
      p2p: {
        libp2p: {
          enabled: false,
        },
        persistent_peers_max_dial_period: '3s',
        pex: false,
      },
    },
    'app-config': {
      chain_id: 'cronos_777-1',
      'app-db-backend': 'goleveldb',
      'minimum-gas-prices': '0basetcro',
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'async-check-tx': true,
      'iavl-lazy-loading': true,
      'json-rpc': {
        address: '127.0.0.1:{EVMRPC_PORT}',
        'ws-address': '127.0.0.1:{EVMRPC_PORT_WS}',
        api: 'eth,net,web3,debug,cronos',
        'feehistory-cap': 100,
        'block-range-cap': 10000,
        'logs-cap': 10000,
        'enable-indexer': false,
      },
      mempool: {
        'max-txs': 100000,
      },
      telemetry: {
        enabled: true,
        'prometheus-retention-time': 60,
      },
      evm: {
        'block-executor': 'block-stm',
        'block-stm-workers': 0,
        'block-stm-pre-estimate': false,
      },
      memiavl: {
        enable: false,
      },
      grpc: {
        'skip-check-header': true,
      },
      cronos: {
        'tx-cache-size': 100000,
      },
    },
    validators: [validator(i) for i in std.range(0, num_validators - 1)],
    accounts: [
      {
        name: 'community',
        coins: '1000000000000000000000000000basetcro',
        mnemonic: '${COMMUNITY_MNEMONIC}',
      },
      {
        name: 'signer1',
        coins: '20000000000000000000000basetcro',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '30000000000000000000000basetcro',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ],
    genesis: {
      consensus: {
        params: {
          block: {
            max_gas: '105000000',
          },
        },
      },
      app_state: {
        evm: {
          params: {
            evm_denom: 'basetcro',
          },
        },
        cronos: {
          params: {
            cronos_admin: '${CRONOS_ADMIN}',
            enable_auto_deployment: true,
            ibc_cro_denom: '${IBC_CRO_DENOM}',
          },
        },
        gov: {
          params: {
            expedited_voting_period: '1s',
            voting_period: '10s',
            max_deposit_period: '10s',
            min_deposit: [
              {
                denom: 'basetcro',
                amount: '1',
              },
            ],
          },
        },
        transfer: {
          params: {
            receive_enabled: true,
            send_enabled: true,
          },
        },
        feemarket: {
          params: {
            no_base_fee: true,
            base_fee: '100000000000',
          },
        },
      },
    },
  },
}
