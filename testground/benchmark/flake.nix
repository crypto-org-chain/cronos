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

  outputs = { self, nixpkgs, flake-utils, poetry2nix }:
    let
      overrides = { lib, poetry2nix }: poetry2nix.overrides.withDefaults
        (self: super:
          let
            buildSystems = {
              pystarport = [ "poetry-core" ];
              durations = [ "setuptools" ];
              multitail2 = [ "setuptools" ];
              docker = [ "hatchling" "hatch-vcs" ];
              pyunormalize = [ "setuptools" ];
            };
          in
          lib.mapAttrs
            (attr: systems: super.${attr}.overridePythonAttrs
              (old: {
                nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
              }))
            buildSystems
        );

      src = nix-gitignore: nix-gitignore.gitignoreSourcePure [
        "/*" # ignore all, then add whitelists
        "!/benchmark/"
        "!poetry.lock"
        "!pyproject.toml"
      ] ./.;

      benchmark = { lib, poetry2nix, python311, nix-gitignore }: poetry2nix.mkPoetryApplication {
        projectDir = src nix-gitignore;
        python = python311;
        overrides = overrides { inherit lib poetry2nix; };
      };

      benchmark-env = { lib, poetry2nix, python311, nix-gitignore }: poetry2nix.mkPoetryEnv {
        projectDir = src nix-gitignore;
        python = python311;
        overrides = overrides { inherit lib poetry2nix; };
      };

    in
    (flake-utils.lib.eachDefaultSystem
      (system:
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
          packages.default = pkgs.testground-testcase;
          apps = {
            default = {
              type = "app";
              program = "${pkgs.testground-testcase}/bin/testground-testcase";
            };
            stateless-testcase = {
              type = "app";
              program = "${pkgs.testground-testcase}/bin/stateless-testcase";
            };
          };
          devShells.default = pkgs.mkShell {
            buildInputs = [ pkgs.testground-testcase-env ];
          };
          legacyPackages = pkgs;
        })
    );
}
