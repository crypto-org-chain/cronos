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
          cronos-config = ../scripts/cronos-devnet.yaml;
          geth-genesis = ../scripts/geth-genesis.json;
        };
      }
    )
    (_: pkgs: {
      orchestrator = pkgs.rustPlatform.buildRustPackage rec {
        name = "orchestrator";
        src = sources.gravity-bridge;
        sourceRoot = "gravity-bridge-src/orchestrator";
        cargoSha256 = sha256:1rii0l2xd2r5ha9ac10da6h2z4wnhf55cp5kdfrfwiacrrg3gg3l;
        cargoBuildFlags = "-p ${name} --features ethermint";
        doCheck = false;
        OPENSSL_NO_VENDOR = "1";
        OPENSSL_DIR = pkgs.symlinkJoin {
          name = "openssl";
          paths = with pkgs.openssl; [ out dev ];
        };
      };
      gorc = pkgs.rustPlatform.buildRustPackage rec {
        name = "gorc";
        src = sources.gravity-bridge;
        sourceRoot = "gravity-bridge-src/orchestrator";
        cargoSha256 = sha256:1rdg71i393hwqw306fqssb7iby3ivxzh42vv53c2gqy90gd9dfjk;
        cargoBuildFlags = "-p ${name} --features ethermint";
        doCheck = false;
        OPENSSL_NO_VENDOR = "1";
        OPENSSL_DIR = pkgs.symlinkJoin {
          name = "openssl";
          paths = with pkgs.openssl; [ out dev ];
        };
      };
    })
    (_: pkgs: {
      test-env = import ./testenv.nix { inherit pkgs; };
    })
  ];
  config = { };
  inherit system;
}
