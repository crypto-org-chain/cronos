final: _:
let
  overrides =
    { lib, poetry2nix }:
    poetry2nix.overrides.withDefaults (
      self: super:
      let
        buildSystems = {
          pystarport = [ "poetry-core" ];
          durations = [ "setuptools" ];
          multitail2 = [ "setuptools" ];
          docker = [
            "hatchling"
            "hatch-vcs"
          ];
          pyunormalize = [ "setuptools" ];
          pytest-github-actions-annotate-failures = [ "setuptools" ];
          cprotobuf = [ "setuptools" ];
          flake8-black = [ "setuptools" ];
          flake8-isort = [ "hatchling" ];
          isort = [ "poetry-core" ];
        };
      in
      lib.mapAttrs (
        attr: systems:
        super.${attr}.overridePythonAttrs (old: {
          nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
        })
      ) buildSystems
      //
        lib.genAttrs
          [
            "eth-hash"
            "eth-keys"
            "eth-keyfile"
            "rlp"
            "web3"
          ]
          (
            name:
            super.${name}.overridePythonAttrs (_: {
              dontConfigure = true;
            })
          )
    )
    ++ [
      # Applied after poetry2nix defaults so the default ckzg postPatch
      # (substituteInPlace src/Makefile) is already evaluated and can be
      # safely replaced. Placing this inside withDefaults doesn't work
      # because defaults run later and the `or` expression breaks.
      (_self: super: {
        ckzg = super.ckzg.overridePythonAttrs (_: {
          postPatch = "";
        });
      })
    ];

  src =
    nix-gitignore:
    nix-gitignore.gitignoreSourcePure [
      "/*" # ignore all, then add whitelists
      "!/benchmark/"
      "!poetry.lock"
      "!pyproject.toml"
    ] ./.;

  benchmark =
    {
      lib,
      poetry2nix,
      python311,
      nix-gitignore,
    }:
    poetry2nix.mkPoetryApplication {
      projectDir = src nix-gitignore;
      python = python311;
      overrides = overrides { inherit lib poetry2nix; };
      preferWheels = true;
    };

  benchmark-env =
    {
      lib,
      poetry2nix,
      python311,
      nix-gitignore,
    }:
    poetry2nix.mkPoetryEnv {
      projectDir = src nix-gitignore;
      python = python311;
      overrides = overrides { inherit lib poetry2nix; };
      preferWheels = true;
    };

in
{
  benchmark-testcase = final.callPackage benchmark { };
  benchmark-testcase-env = final.callPackage benchmark-env { };
}
