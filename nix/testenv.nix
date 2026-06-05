{
  poetry2nix,
  lib,
  python311,
  pkgs,
}:
let
  # crates.io rejects the default "python-requests" User-Agent with HTTP 403
  # (see https://crates.io/data-access), which makes fetchCargoVendor's crate
  # downloads fail. Patch the `requests` used by the vendoring tool to send a
  # descriptive User-Agent. This is scoped to fetchCargoVendor's downloader, so
  # no other Python packages rebuild, and the vendored output hash is unchanged.
  fetchCargoVendor = pkgs.rustPlatform.fetchCargoVendor.override {
    python3Packages = pkgs.python3Packages.overrideScope (
      _: prev: {
        requests = prev.requests.overridePythonAttrs (old: {
          postPatch = (old.postPatch or "") + ''
            substituteInPlace src/requests/utils.py \
              --replace-fail 'name="python-requests"' 'name="cronos-cargo-vendor"'
          '';
        });
      }
    );
  };

  # Override the default poetry2nix overrides to fix rpds-py
  customPoetry2nix = poetry2nix.overrideScope (
    final: prev: {
      defaultPoetryOverrides = prev.defaultPoetryOverrides.extend (
        self: super: {
          rpds-py = super.rpds-py.overridePythonAttrs (
            old:
            lib.optionalAttrs (!(old.src.isWheel or false)) {
              cargoDeps = fetchCargoVendor {
                inherit (old) src pname version;
                hash = "sha256-npvJz6PMHWzPkI0LVNeiMsZVxmwR6uzjlhBPMCCrFfw=";
              };
              nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [
                pkgs.rustPlatform.cargoSetupHook
                pkgs.cargo
                pkgs.rustc
              ];
            }
          );
        }
      );
    }
  );
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
