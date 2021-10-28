{ sources ? import ./sources.nix, system ? builtins.currentSystem }:

import sources.nixpkgs {
  overlays = [
    (import (sources.dapptools + "/overlay.nix"))
    (import (sources.gomod2nix + "/overlay.nix"))
    (import (sources.poetry2nix + "/overlay.nix"))
    (_: pkgs: {
      pystarport = pkgs.poetry2nix.mkPoetryApplication {
        projectDir = sources.pystarport;
        src = sources.pystarport;
      };
    })
    (_: pkgs:
      import ./scripts.nix {
        inherit pkgs;
        config = {
          chainmain-config = ../scripts/chainmain-devnet.yaml;
          cronos-config = ../scripts/cronos-devnet.yaml;
          hermes-config = ../scripts/hermes.toml;
          geth-genesis = ../scripts/geth-genesis.json;
        };
      })
    (_: pkgs: {
      gorc = pkgs.rustPlatform.buildRustPackage rec {
        name = "gorc";
        src = sources.gravity-bridge;
        sourceRoot = "gravity-bridge-src/orchestrator";
        cargoSha256 =
          "sha256:08bpbi7j0jr9mr65hh92gcxys5yqrgyjx6fixjg4v09yyw5im9x7";
        cargoBuildFlags = "-p ${name} --features ethermint";
        buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin
          [ pkgs.darwin.apple_sdk.frameworks.Security ];
        doCheck = false;
        OPENSSL_NO_VENDOR = "1";
        OPENSSL_DIR = pkgs.symlinkJoin {
          name = "openssl";
          paths = with pkgs.openssl; [ out dev ];
        };
      };
    })
    (_: pkgs: { test-env = import ./testenv.nix { inherit pkgs; }; })
  ];
  config = { };
  inherit system;
}
