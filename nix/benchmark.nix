{ lib, poetry2nix, python311, nix-gitignore }:
let
  overrides = poetry2nix.overrides.withDefaults
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
in
poetry2nix.mkPoetryApplication {
  projectDir = nix-gitignore.gitignoreSourcePure [
    "/*" # ignore all, then add whitelists
    "!/benchmark/"
    "!poetry.lock"
    "!pyproject.toml"
  ] ../testground/benchmark;
  python = python311;
  inherit overrides;
}

