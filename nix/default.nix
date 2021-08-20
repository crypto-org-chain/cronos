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
        cargoSha256 = sha256:06s3qr4nx6ksmg83c9wvpbp8lvqnrklhwg2gazs5frc6qv63llm3;
        cargoBuildFlags = "-p ${name} --features ethermint";
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
