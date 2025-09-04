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
        postPatch =
          (old.postPatch or "")
          + ''
            sed -i '/^license-files = \["LICENSE"\]$/d' pyproject.toml
            substituteInPlace pyproject.toml \
              --replace-warn 'license = "PSF-2.0"' 'license = { text = "PSF-2.0" }'
          '';
      });
      types-requests = super.types-requests.overridePythonAttrs (old: {
        postPatch =
          (old.postPatch or "")
          + ''
            sed -i '/^license-files = \["LICENSE"\]$/d' pyproject.toml
            substituteInPlace pyproject.toml \
              --replace-warn 'license = "Apache-2.0"' 'license = { text = "Apache-2.0" }'
          '';
      });
    }
  );
}
