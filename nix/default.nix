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
    (import "${sources.poetry2nix}/overlay.nix")
    (_: pkgs: {
      go = pkgs.go_1_17;
      go-ethereum = pkgs.callPackage ./go-ethereum.nix {
        inherit (pkgs.darwin) libobjc;
        inherit (pkgs.darwin.apple_sdk.frameworks) IOKit;
        buildGoModule = pkgs.buildGo117Module;
      };
      flake-compat = import sources.flake-compat;
    }) # update to a version that supports eip-1559
    (import (sources.gomod2nix + "/overlay.nix"))
    (_: pkgs: {
      pystarport = pkgs.poetry2nix.mkPoetryApplication rec {
        projectDir = sources.pystarport;
        src = projectDir;
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
          dotenv = builtins.path { name = "dotenv"; path = ../scripts/.env; };
        };
      })
    (_: pkgs: {
      gorc = pkgs.rustPlatform.buildRustPackage rec {
        name = "gorc";
        src = sources.gravity-bridge;
        sourceRoot = "gravity-bridge-src/orchestrator";
        cargoSha256 = "sha256-OX/cG4p6XGZX85QxmDH/uTvGqvnV+B6TWEL3fyk5/zc=";
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
    })
    (_: pkgs: { test-env = import ./testenv.nix { inherit pkgs; }; })
    (_: pkgs: {
      rocksdb = pkgs.rocksdb.overrideAttrs (old: rec {
        pname = "rocksdb";
        version = "6.27.3";
        src = sources.rocksdb;
      });
    })
    (_: pkgs: {
      cosmovisor = pkgs.buildGo117Module rec {
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
