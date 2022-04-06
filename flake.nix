{
  inputs = {
    nixpkgs = {
      url = "path:./nix";
      flake = false;
    };
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    (flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            inherit system;
          };
        in
        rec {
          packages = {
            cronosd = pkgs.callPackage ./. { };
            cronosd-testnet = pkgs.callPackage ./. { network = "testnet"; };
          };
          apps = {
            cronosd = {
              type = "app";
              program = "${packages.cronosd}/bin/cronosd";
            };
            cronosd-testnet = {
              type = "app";
              program = "${packages.cronosd-testnet}/bin/cronosd";
            };
          };
          defaultPackage = packages.cronosd;
          defaultApp = apps.cronosd;
          devShells = {
            cronosd = pkgs.mkShell {
              buildInputs = with pkgs; [
                go
                rocksdb
                gomod2nix
              ];
            };
          };
          devShell = devShells.cronosd;
        }
      )
    );
}
