{ poetry2nix, python310, lib }:
poetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  python = python310;
  overrides = poetry2nix.overrides.withDefaults (lib.composeManyExtensions [
    (self: super:
      let
        buildSystems = {
          cprotobuf = [ "setuptools" ];
          durations = [ "setuptools" ];
          multitail2 = [ "setuptools" ];
          pytest-github-actions-annotate-failures = [ "setuptools" ];
          flake8-black = [ "setuptools" ];
          multiaddr = [ "setuptools" ];
        };
      in
      lib.mapAttrs
        (attr: systems: super.${attr}.overridePythonAttrs
          (old: {
            nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
          }))
        buildSystems
    )
    (self: super: {
      pyyaml-include = super.pyyaml-include.overridePythonAttrs {
        preConfigure = ''
          substituteInPlace setup.py --replace "setup()" "setup(version=\"1.3\")"
        '';
      };
    })
  ]);
}
