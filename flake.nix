{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/release-24.11";
    flake-utils.url = "github:numtide/flake-utils";
    nix-bundle-exe = {
      url = "github:3noch/nix-bundle-exe";
      flake = false;
    };
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
    poetry2nix = {
      url = "github:nix-community/poetry2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      nix-bundle-exe,
      gomod2nix,
      flake-utils,
      poetry2nix,
    }:
    let
      rev = self.shortRev or "dirty";
      mkApp = drv: {
        type = "app";
        program = "${drv}/bin/${drv.meta.mainProgram}";
      };
    in
    (flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = self.overlays.default;
          config = { };
        };
      in
      rec {
        packages = pkgs.cronos-matrix // {
          inherit (pkgs) rocksdb testground-image;
        };
        apps = {
          cronosd = mkApp packages.cronosd;
          cronosd-testnet = mkApp packages.cronosd-testnet;
          stateless-testcase = {
            type = "app";
            program = "${pkgs.benchmark-testcase}/bin/stateless-testcase";
          };
        };
        defaultPackage = packages.cronosd;
        defaultApp = apps.cronosd;
        devShells = rec {
          default = pkgs.mkShell {
            buildInputs = [
              defaultPackage.go
              pkgs.gomod2nix
              pkgs.nixfmt-rfc-style
            ];
          };
          rocksdb = pkgs.mkShell {
            buildInputs = default.buildInputs ++ [
              pkgs.rocksdb
              pkgs.rocksdb.tools
            ];
          };
          full = pkgs.mkShell { buildInputs = rocksdb.buildInputs ++ [ pkgs.test-env ]; };
        };
        legacyPackages = pkgs;
      }
    ))
    // {
      overlays.default = [
        (import ./nix/build_overlay.nix)
        poetry2nix.overlays.default
        gomod2nix.overlays.default
        (import ./testground/benchmark/overlay.nix)
        (final: super: {
          go = super.go_1_23;
          test-env = final.callPackage ./nix/testenv.nix { };
          cronos-matrix = final.callPackage ./nix/cronos-matrix.nix {
            inherit rev;
            bundle-exe = final.pkgsBuildBuild.callPackage nix-bundle-exe { };
          };
          testground-image = final.callPackage ./nix/testground-image.nix { };
        })
      ];
    };
}
