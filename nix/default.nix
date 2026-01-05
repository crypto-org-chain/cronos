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
    (final: pkgs: {
      go = final.go_1_25;
      go-ethereum = pkgs.callPackage ./go-ethereum.nix {
        buildGoModule = pkgs.buildGoModule;
      };
      flake-compat = import sources.flake-compat;
      chain-maind =
        (pkgs.callPackage sources.chain-main {
          rocksdb = null;
          buildPackages = pkgs.buildPackages // {
            go_1_23 = final.buildPackages.go_1_25;
          };
        }).overrideAttrs
          (old: {
            # Fix modRoot issue - gomod2nix builder needs modRoot set to non-null
            # See: https://github.com/crypto-org-chain/chain-main/pull/1220
            modRoot = ".";
          });
    })
    (import "${sources.poetry2nix}/overlay.nix")
    (
      final: prev:
      let
        gomodSrc = sources.gomod2nix;
        callPackage = final.callPackage;
      in
      {
        inherit (callPackage "${gomodSrc}/builder" { }) buildGoApplication mkGoEnv mkVendorEnv;
        gomod2nix = (callPackage "${gomodSrc}/default.nix" { }).overrideAttrs (_: {
          modRoot = ".";
        });
      }
    )
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
        buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin [
          pkgs.apple-sdk_15
        ];
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
    (_: pkgs: { hermes = pkgs.callPackage ./hermes.nix { }; })
    (_: pkgs: { test-env = pkgs.callPackage ./testenv.nix { inherit pkgs; }; })
    (_: pkgs: { cosmovisor = pkgs.callPackage ./cosmovisor.nix { }; })
    (_: pkgs: {
      rly = pkgs.buildGoModule rec {
        name = "rly";
        src = sources.relayer;
        subPackages = [ "." ];
        vendorHash = "sha256-dwKZZu9wKOo2u1/8AAWFx89iC9pWZbCxAERMMAOFsts=";
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
