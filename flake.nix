{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/master";
    flake-utils.url = "github:numtide/flake-utils";
    nix-bundle-exe = {
      url = "github:3noch/nix-bundle-exe";
      flake = false;
    };
    gomod2nix = {
      url = "github:yihuang/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.utils.follows = "flake-utils";
    };
  };

  outputs = { self, nixpkgs, nix-bundle-exe, gomod2nix, flake-utils }:
    let
      rev = self.shortRev or "dirty";
      mkApp = drv: {
        type = "app";
        program = "${drv}/bin/${drv.meta.mainProgram}";
      };
    in
    (flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [
              gomod2nix.overlays.default
              self.overlay
            ];
            config = { };
          };
        in
        rec {
          packages = pkgs.cronos-matrix // {
            inherit (pkgs) rocksdb;
          };
          apps = {
            cronosd = mkApp packages.cronosd;
            cronosd-testnet = mkApp packages.cronosd-testnet;
          };
          defaultPackage = packages.cronosd;
          defaultApp = apps.cronosd;
          devShells = {
            cronosd = pkgs.mkShell {
              buildInputs = with pkgs; [
                go_1_18
                rocksdb
                gomod2nix
              ];
            };
          };
          devShell = devShells.cronosd;
        }
      )
    ) // {
      overlay = final: _: {
        bundle-exe = import nix-bundle-exe { pkgs = final; };
        # make-tarball don't follow symbolic links to avoid duplicate file, the bundle should have no external references.
        # reset the ownership and permissions to make the extract result more normal.
        make-tarball = drv: with final; runCommand drv.name { } ''
          "${gnutar}/bin/tar" cfv - -C ${drv} \
            --owner=0 --group=0 --mode=u+rw,uga+r --hard-dereference . \
            | "${gzip}/bin/gzip" -9 > $out
        '';
        rocksdb = final.callPackage ./nix/rocksdb.nix { enableJemalloc = true; };
      } // (with final;
        let
          matrix = lib.cartesianProductOfSets {
            network = [ "mainnet" "testnet" ];
            pkgtype = [
              "nix" # normal nix package
              "bundle" # relocatable bundled package
              "tarball" # tarball of the bundle, for distribution and checksum
            ];
          };
          binaries = builtins.listToAttrs (builtins.map
            ({ network, pkgtype }: {
              name = builtins.concatStringsSep "-" (
                [ "cronosd" ] ++
                lib.optional (network != "mainnet") network ++
                lib.optional (pkgtype != "nix") pkgtype
              );
              value =
                let
                  cronosd = callPackage ./. {
                    inherit rev network;
                  };
                  bundle = bundle-exe cronosd;
                in
                if pkgtype == "bundle" then
                  bundle
                else if pkgtype == "tarball" then
                  make-tarball bundle
                else
                  cronosd;
            })
            matrix
          );
        in
        {
          cronos-matrix = binaries;
        }
      );
    };
}
