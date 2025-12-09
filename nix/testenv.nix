{
  poetry2nix,
  lib,
  python311,
  pkgs,
}:
let
  # Override the default poetry2nix overrides to fix rpds-py
  customPoetry2nix = poetry2nix.overrideScope (final: prev: {
    defaultPoetryOverrides = prev.defaultPoetryOverrides.extend (self: super: {
      rpds-py = super.rpds-py.overridePythonAttrs (old:
        lib.optionalAttrs (!(old.src.isWheel or false)) {
          cargoDeps = pkgs.rustPlatform.fetchCargoVendor {
            inherit (old) src pname version;
            hash = "sha256-0wMmhiUjXY5DaA43l7kBKE7IX1UoEFZBJ8xnafVlU60=";
          };
          nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [
            pkgs.rustPlatform.cargoSetupHook
            pkgs.cargo
            pkgs.rustc
          ];
        }
      );
    });
  });
in
customPoetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  python = python311;
  overrides = customPoetry2nix.overrides.withDefaults (
    self: super:
    let
      buildSystems = {
        pystarport = [ "poetry-core" ];
        cprotobuf = [
          "setuptools"
          "poetry-core"
        ];
        durations = [ "setuptools" ];
        multitail2 = [ "setuptools" ];
        pytest-github-actions-annotate-failures = [ "setuptools" ];
        flake8-black = [ "setuptools" ];
        flake8-isort = [ "hatchling" ];
        pyunormalize = [ "setuptools" ];
        eth-bloom = [ "setuptools" ];
        isort = [ "poetry-core" ];
      };
    in
    (lib.mapAttrs (
      attr: systems:
      super.${attr}.overridePythonAttrs (old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
      })
    ) buildSystems)
    // {
      typing-extensions = super.typing-extensions.overridePythonAttrs (old: {
        postPatch = (old.postPatch or "") + ''
          sed -i '/^license-files = \["LICENSE"\]$/d' pyproject.toml
          substituteInPlace pyproject.toml \
            --replace-warn 'license = "PSF-2.0"' 'license = { text = "PSF-2.0" }'
        '';
      });
      types-requests = super.types-requests.overridePythonAttrs (old: {
        postPatch = (old.postPatch or "") + ''
          sed -i '/^license-files = \["LICENSE"\]$/d' pyproject.toml
          substituteInPlace pyproject.toml \
            --replace-warn 'license = "Apache-2.0"' 'license = { text = "Apache-2.0" }'
        '';
      });
    }
  );
}
