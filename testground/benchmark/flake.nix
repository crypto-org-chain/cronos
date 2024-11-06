{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
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
      flake-utils,
      poetry2nix,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [
            poetry2nix.overlays.default
            (import ./overlay.nix)
          ];
          config = { };
        };
      in
      rec {
        packages.default = pkgs.benchmark-testcase;
        apps = {
          default = {
            type = "app";
            program = "${pkgs.benchmark-testcase}/bin/stateless-testcase";
          };
          stateless-testcase = {
            type = "app";
            program = "${pkgs.benchmark-testcase}/bin/stateless-testcase";
          };
          testnet = {
            type = "app";
            program = "${pkgs.benchmark-testcase}/bin/testnet";
          };
        };
        devShells.default = pkgs.mkShell {
          buildInputs = [ pkgs.benchmark-testcase-env ];
        };
        legacyPackages = pkgs;
      }
    );
}
