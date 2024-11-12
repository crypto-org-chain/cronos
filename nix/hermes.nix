{
  src,
  lib,
  stdenv,
  darwin,
  rustPlatform,
  symlinkJoin,
  openssl,
  pkg-config,
}:

rustPlatform.buildRustPackage rec {
  name = "hermes";
  inherit src;
  cargoBuildFlags = "-p ibc-relayer-cli";
  buildInputs = lib.optionals stdenv.isDarwin [
    darwin.apple_sdk.frameworks.Security
    pkg-config
    openssl
    darwin.libiconv
    darwin.apple_sdk.frameworks.SystemConfiguration
  ];
  cargoLock = {
    lockFile = "${src}/Cargo.lock";
    outputHashes = {
      "ibc-proto-0.38.0" = "sha256-UhpWBzraC6fMPJ0BVK6CxdrywoEayNq0tBU0N3MxmB4=";
    };
  };
  doCheck = false;
  RUSTFLAGS = "--cfg ossl111 --cfg ossl110 --cfg ossl101";
  OPENSSL_NO_VENDOR = "1";
  OPENSSL_DIR = symlinkJoin {
    name = "openssl";
    paths = with openssl; [
      out
      dev
    ];
  };
}
