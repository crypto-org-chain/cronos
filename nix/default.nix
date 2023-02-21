{ sources ? import ./sources.nix, system ? builtins.currentSystem, ... }:

let
  dapptools = {
    x86_64-linux =
      (import (sources.dapptools + "/release.nix") { }).dapphub.linux.stable;
    x86_64-darwin =
      (import (sources.dapptools + "/release.nix") { }).dapphub.darwin.stable;
  }.${system} or (throw
    "Unsupported system: ${system}");
in
import sources.nixpkgs {
  overlays = [
    (_: pkgs: dapptools) # use released version to hit the binary cache
    (final: pkgs: rec {
      go_1_19 = pkgs.go_1_19.overrideAttrs (_: rec {
        version = "1.19.6";
        src = final.fetchurl {
          url = "https://go.dev/dl/go${version}.src.tar.gz";
          hash = "sha256-1/ABP4Lm1/hizGy1yM20ju9fLiObNbqpfi8adGYEN2c=";
        };
      });
      go = go_1_19;
      go-ethereum = pkgs.callPackage ./go-ethereum.nix {
        inherit (pkgs.darwin) libobjc;
        inherit (pkgs.darwin.apple_sdk.frameworks) IOKit;
        buildGoModule = pkgs.buildGo118Module;
      };
      flake-compat = import sources.flake-compat;
      chain-maind = pkgs.callPackage sources.chain-main { rocksdb = null; };
    }) # update to a version that supports eip-1559
    (import "${sources.gomod2nix}/overlay.nix")
    (pkgs: _:
      import ./scripts.nix {
        inherit pkgs;
        config = {
          cronos-config = ../scripts/cronos-devnet.yaml;
          geth-genesis = ../scripts/geth-genesis.json;
          dotenv = builtins.path { name = "dotenv"; path = ../scripts/.env; };
        };
      })
    (_: pkgs: {
      gorc = pkgs.rustPlatform.buildRustPackage rec {
        name = "gorc";
        src = sources.gravity-bridge;
        sourceRoot = "gravity-bridge-src/orchestrator";
        cargoSha256 = "sha256-FQ43PFGbagIi+KZ6KUtjF7OClIkCqKd4pGzHaYr2Q+A=";
        cargoBuildFlags = "-p ${name} --features ethermint";
        buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin
          (with pkgs.darwin.apple_sdk.frameworks; [ CoreFoundation Security ]);
        doCheck = false;
        OPENSSL_NO_VENDOR = "1";
        OPENSSL_DIR = pkgs.symlinkJoin {
          name = "openssl";
          paths = with pkgs.openssl; [ out dev ];
        };
      };
      hermes = pkgs.callPackage ./hermes.nix { src = sources.ibc-rs; };
    })
    (_: pkgs: { test-env = pkgs.callPackage ./testenv.nix { }; })
    (final: _: {
      rocksdb = final.callPackage ./rocksdb.nix { enableJemalloc = true; };
    })
    (_: pkgs: {
      cosmovisor = pkgs.buildGo118Module rec {
        name = "cosmovisor";
        src = sources.cosmos-sdk + "/cosmovisor";
        subPackages = [ "./cmd/cosmovisor" ];
        vendorSha256 = "sha256-OAXWrwpartjgSP7oeNvDJ7cTR9lyYVNhEM8HUnv3acE=";
        doCheck = false;
      };
    })
  ];
  config = { };
  inherit system;
}
