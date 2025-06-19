{
  sources ? import ./sources.nix,
  system ? builtins.currentSystem,
  ...
}:
import sources.nixpkgs {
  overlays = [
    (import ./build_overlay.nix)
    (import ./dapptools-overlay.nix {
      dapptools-release = sources.dapptools;
      dapptools-master = sources.dapptools-master;
    })
    (_: pkgs: {
      go = pkgs.go_1_22;
      go-ethereum = pkgs.callPackage ./go-ethereum.nix {
        inherit (pkgs.darwin) libobjc;
        inherit (pkgs.darwin.apple_sdk.frameworks) IOKit;
        buildGoModule = pkgs.buildGo122Module;
      };
      flake-compat = import sources.flake-compat;
      chain-maind = pkgs.callPackage sources.chain-main { rocksdb = null; };
    }) # update to a version that supports eip-1559
    (import "${sources.poetry2nix}/overlay.nix")
    (import "${sources.gomod2nix}/overlay.nix")
    (
      pkgs: _:
      import ./scripts.nix {
        inherit pkgs;
        config = {
          cronos-config = ../scripts/cronos-devnet.yaml;
          geth-genesis = ../scripts/geth-genesis.json;
          dotenv = builtins.path {
            name = "dotenv";
            path = ../scripts/.env;
          };
        };
      }
    )
    (_: pkgs: {
      gorc = pkgs.rustPlatform.buildRustPackage rec {
        name = "gorc";
        src = sources.gravity-bridge;
        sourceRoot = "gravity-bridge-src/orchestrator";
        cargoSha256 = "sha256-FQ43PFGbagIi+KZ6KUtjF7OClIkCqKd4pGzHaYr2Q+A=";
        cargoBuildFlags = "-p ${name} --features ethermint";
        buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin (
          with pkgs.darwin.apple_sdk.frameworks;
          [
            CoreFoundation
            Security
          ]
        );
        doCheck = false;
        OPENSSL_NO_VENDOR = "1";
        OPENSSL_DIR = pkgs.symlinkJoin {
          name = "openssl";
          paths = with pkgs.openssl; [
            out
            dev
          ];
        };
      };
    })
    (_: pkgs: {
      hermes =
        let
          # The informalsystems/hermes v1.13.1 requires rust version >= v1.83
          # The nixpkgs 24.11 is using rust version v1.82
          # Use fenix to select different rust toolchain version
          rustToolchain =
            (import sources.fenix {
              inherit system;
            }).fromToolchainFile
              {
                file = ./rust-toolchain.toml;
                sha256 = "sha256-s1RPtyvDGJaX/BisLT+ifVfuhDT1nZkZ1NcK8sbwELM=";
              };
          fenixRustPlatform = pkgs.makeRustPlatform {
            cargo = rustToolchain;
            rustc = rustToolchain;
          };
        in
        pkgs.callPackage ./hermes.nix {
          src = sources.hermes;
          rustPlatform = fenixRustPlatform;
        };
    })
    (_: pkgs: { test-env = pkgs.callPackage ./testenv.nix { }; })
    (_: pkgs: { cosmovisor = pkgs.callPackage ./cosmovisor.nix { }; })
    (_: pkgs: {
      rly = pkgs.buildGo123Module rec {
        name = "rly";
        src = sources.relayer;
        subPackages = [ "." ];
        vendorHash = "sha256-O8bjUfB+tXDizb4uKfpE+A3roFDjD8AYba8ncTAHlF0=";
        doCheck = false;
        env.GOWORK = "off";
        postInstall = ''
          mv $out/bin/relayer $out/bin/rly
        '';
      };
    })
  ];
  config = { };
  inherit system;
}
