{
  lib,
  stdenv,
  rustPlatform,
  apple-sdk_15,
  sources,
}:

# Built from the crypto-org-chain fork: upstream releases can't decode ML-DSA-65
# validator keys (tendermint-rs backport of cometbft/tendermint-rs#1533).
rustPlatform.buildRustPackage {
  pname = "hermes";
  version = "1.13.3-mldsa65";
  src = sources.hermes;
  # tendermint-rs is a git dependency pinned by commit in Cargo.lock
  cargoLock = {
    lockFile = "${sources.hermes}/Cargo.lock";
    allowBuiltinFetchGit = true;
  };
  cargoBuildFlags = [
    "-p"
    "ibc-relayer-cli"
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ apple-sdk_15 ];
  doCheck = false;

  meta = with lib; {
    description = "An IBC Relayer written in Rust";
    homepage = "https://github.com/crypto-org-chain/hermes";
    license = licenses.asl20;
    platforms = platforms.unix;
    mainProgram = "hermes";
  };
}
