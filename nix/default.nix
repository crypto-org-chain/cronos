{ sources ? import ./sources.nix, system ? builtins.currentSystem, ... }:
import sources.nixpkgs {
  overlays = [
    (import ./build_overlay.nix)
    (import ./dapptools-overlay.nix { dapptools-release = sources.dapptools; dapptools-master = sources.dapptools-master; })
    (_: pkgs: {
      go = pkgs.go_1_22;
      go-ethereum = pkgs.callPackage ./go-ethereum.nix {
        inherit (pkgs.darwin) libobjc;
        inherit (pkgs.darwin.apple_sdk.frameworks) IOKit;
        buildGoModule = pkgs.buildGo120Module;
      };
      flake-compat = import sources.flake-compat;
      chain-maind = pkgs.callPackage sources.chain-main { rocksdb = null; };
    }) # update to a version that supports eip-1559
    (import "${sources.poetry2nix}/overlay.nix")
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
    (_: pkgs: {
      cosmovisor = pkgs.buildGo120Module rec {
        name = "cosmovisor";
        src = sources.cosmos-sdk + "/cosmovisor";
        subPackages = [ "./cmd/cosmovisor" ];
        vendorHash = "sha256-OAXWrwpartjgSP7oeNvDJ7cTR9lyYVNhEM8HUnv3acE=";
        doCheck = false;
      };
    })
    (_: pkgs: {
      rly = pkgs.buildGo120Module rec {
        name = "rly";
        src = sources.relayer;
        subPackages = [ "." ];
        vendorHash = "sha256-5s5URiAZk59mOfZdUcqYAB/IvzFuBiQH5jDBuh6huTE=";
        doCheck = false;
        GOWORK = "off";
        postInstall = ''
          mv $out/bin/relayer $out/bin/rly
        '';
      };
    })
  ];
  config = { };
  inherit system;
}
