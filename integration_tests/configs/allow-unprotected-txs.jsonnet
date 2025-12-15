local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      'json-rpc'+: {
        // Ethermint server config: enable sending non-EIP155 (unprotected) txs over RPC.
        'allow-unprotected-txs': true,
      },
    },
    genesis+: {
      app_state+: {
        // Allow legacy/unprotected txs over JSON-RPC (replay txs without EIP-155)
        evm+: {
          params+: {
            allow_unprotected_txs: true,
          },
        },
        // The replay tx fixture uses gasPrice=10 gwei. Disable base fee so it can be mined.
        feemarket+: {
          params+: {
            no_base_fee: true,
            base_fee: '0',
          },
        },
      },
    },
  },
}


