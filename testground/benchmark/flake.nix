{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    cronos = {
      url = "github:crypto-org-chain/cronos/main";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
      inputs.poetry2nix.url = "github:nix-community/poetry2nix";
    };
  };

  outputs = { self, nixpkgs, flake-utils, cronos }:
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

      image = { dockerTools, cronos-matrix, test-env, benchmark }:
        dockerTools.buildLayeredImage {
          name = "cronos-testground";
          contents = [
            benchmark
            test-env
            cronos-matrix.cronosd
          ];
          config = {
            Expose = [ 9090 26657 26656 1317 26658 26660 26659 30000 ];
            Cmd = [ "/bin/benchmark" ];
            Env = [
              "PYTHONUNBUFFERED=1"
            ];
          };
        };
    in
    (flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = cronos.overlays.default ++ [
              (final: _: {
                benchmark = final.callPackage benchmark { };
                benchmark-env = final.callPackage benchmark-env { };
                benchmark-image = final.callPackage image { };
              })
            ];
            config = { };
          };
        in
        rec {
          packages.default = pkgs.benchmark-image;
          devShells.default = pkgs.mkShell {
            buildInputs = [ pkgs.benchmark-env ];
          };
          legacyPackages = pkgs;
        })
    );
}
