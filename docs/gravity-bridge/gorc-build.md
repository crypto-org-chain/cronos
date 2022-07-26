## Gravity Orchestrator (gorc)

gorc, short for gravity orchestrator, is the main process to run gravity bridge orchestrators (for validators) and relayers.

This guide shows the build process for gorc.

### Prerequisites

 - rust & cargo [>=1.61] (can be installed [here](https://www.rust-lang.org/tools/install)). They must be included in $PATH variable

### Build

In order to build the binary, we will need to first clone the repository:

```bash
git clone https://github.com/crypto-org-chain/gravity-bridge.git
cd gravity-bridge
git checkout v2.0.0-cronos-alpha0
```

Then, we can build with cargo:

```bash
cd orchestrator
cargo build --release --features ethermint
```

After the build is complete, you will find the binary at `./target/release/gorc`
