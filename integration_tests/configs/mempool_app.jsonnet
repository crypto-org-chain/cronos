// Exercises CometBFT v0.39 app-side mempool (mempool.type=app) on cronos.
//
// Cronos overrides ReapTxs (MaxBytes/MaxGas cap) and uses cosmos-sdk's
// default InsertTx, which runs the AnteHandler chain via RunTx(execModeCheck)
// before mempool admission. Peer-relayed txs are validated equivalently to
// RPC CheckTx.
local default = import 'default.jsonnet';

default {
  'cronos_777-1'+: {
    config+: {
      mempool+: {
        type: 'app',
        // CometBFT requires reap_interval > 0 when type=app.
        reap_interval: '500ms',
      },
      consensus+: {
        timeout_commit: '5s',
      },
    },
  },
}
