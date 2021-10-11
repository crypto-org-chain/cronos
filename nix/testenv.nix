{ pkgs }:
pkgs.poetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  overrides = pkgs.poetry2nix.overrides.withDefaults (self: super: {
    eth-bloom = super.eth-bloom.overridePythonAttrs {
      preConfigure = ''
        substituteInPlace setup.py --replace \'setuptools-markdown\' ""
      '';
    };
  });
}
