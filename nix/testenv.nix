{
  poetry2nix,
  lib,
  python311,
}:
poetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  python = python311;
  overrides = poetry2nix.overrides.withDefaults (
    self: super:
    let
      buildSystems = {
        pystarport = [ "poetry-core" ];
        cprotobuf = [ "setuptools" "poetry-core" ];
        durations = [ "setuptools" ];
        multitail2 = [ "setuptools" ];
        pytest-github-actions-annotate-failures = [ "setuptools" ];
        flake8-black = [ "setuptools" ];
        flake8-isort = [ "hatchling" ];
        pyunormalize = [ "setuptools" ];
        eth-bloom = [ "setuptools" ];
        isort = [ "poetry-core" ];
        docker = [ "hatchling" "hatch-vcs" ];
      };
    in
    lib.mapAttrs (
      attr: systems:
      super.${attr}.overridePythonAttrs (old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
      })
    ) buildSystems
  );
}
