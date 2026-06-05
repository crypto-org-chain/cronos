{
  poetry2nix,
  lib,
  python311,
  pkgs,
}:
let
  # Override the default poetry2nix overrides to fix rpds-py and ckzg wheel builds
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
          # When preferWheels=true selects the ckzg wheel, the default poetry2nix
          # override's postPatch tries to patch src/Makefile which doesn't exist
          # in the wheel. Clear postPatch for wheel builds.
          ckzg = super.ckzg.overridePythonAttrs (
            old: lib.optionalAttrs (old.src.isWheel or false) { postPatch = ""; }
          );
          # eth-hash, eth-keyfile, eth-keys, web3, rlp: default overrides run
          # substituteInPlace setup.py in preConfigure, but setup.py doesn't
          # exist in wheel distributions (preferWheels=true).
          eth-hash = super.eth-hash.overridePythonAttrs (
            old: lib.optionalAttrs (old.src.isWheel or false) { preConfigure = ""; }
          );
          rlp = super.rlp.overridePythonAttrs (
            old: lib.optionalAttrs (old.src.isWheel or false) { preConfigure = ""; }
          );
          eth-keyfile = super.eth-keyfile.overridePythonAttrs (
            old: lib.optionalAttrs (old.src.isWheel or false) { preConfigure = ""; }
          );
          eth-keys = super.eth-keys.overridePythonAttrs (
            old: lib.optionalAttrs (old.src.isWheel or false) { preConfigure = ""; }
          );
          web3 = super.web3.overridePythonAttrs (
            old: lib.optionalAttrs (old.src.isWheel or false) { preConfigure = ""; }
          );
        }
      );
    }
  );
in
customPoetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  python = python311;
  preferWheels = true;
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
      typing-extensions = super.typing-extensions.overridePythonAttrs (
        old:
        lib.optionalAttrs (!(old.src.isWheel or false)) {
          postPatch = (old.postPatch or "") + ''
            sed -i '/^license-files = \["LICENSE"\]$/d' pyproject.toml
            substituteInPlace pyproject.toml \
              --replace-warn 'license = "PSF-2.0"' 'license = { text = "PSF-2.0" }'
          '';
        }
      );
      types-requests = super.types-requests.overridePythonAttrs (
        old:
        lib.optionalAttrs (!(old.src.isWheel or false)) {
          postPatch = (old.postPatch or "") + ''
            sed -i '/^license-files = \["LICENSE"\]$/d' pyproject.toml
            substituteInPlace pyproject.toml \
              --replace-warn 'license = "Apache-2.0"' 'license = { text = "Apache-2.0" }'
          '';
        }
      );
    }
  );
}
