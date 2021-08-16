{ sources ? import ./sources.nix, system ? builtins.currentSystem }:

import sources.nixpkgs {
  overlays = [
    (import (sources.gomod2nix + "/overlay.nix"))
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
        sourceRoot = "gravity-bridge-src/${name}";
        cargoSha256 = sha256:1sp1f959qzigrpxi3qz33sr4cbl7s805p72nhv7gymjzjscr578z;
        cargoBuildFlags = "-p ${name}";
        doCheck = false;
        OPENSSL_NO_VENDOR = "1";
        OPENSSL_DIR = pkgs.symlinkJoin {
          name = "openssl";
          paths = with pkgs.openssl; [ out dev ];
        };
      };
    })
  ];
  config = { };
  inherit system;
}
