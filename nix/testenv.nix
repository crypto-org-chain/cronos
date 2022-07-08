{ pkgs }:
pkgs.poetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  overrides = pkgs.poetry2nix.overrides.withDefaults (self: super: {
    eth-bloom = super.eth-bloom.overridePythonAttrs {
      preConfigure = ''
        substituteInPlace setup.py --replace \'setuptools-markdown\' ""
      '';
    };

    # https://github.com/nix-community/poetry2nix/issues/218#issuecomment-981615612
    tomli = super.tomli.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.flit-core ];
      }
    );

    typing-extensions = super.typing-extensions.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.flit-core ];
      }
    );

    platformdirs = pkgs.python3Packages.platformdirs;

    black = super.black.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.flit-core ];
        buildInputs = (old.buildInputs or [ ]) ++ [ self.platformdirs ];
      }
    );

    pyparsing = super.pyparsing.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.flit-core ];
      }
    );

    jsonschema = super.jsonschema.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.hatchling self.hatch-vcs ];
      }
    );

    pystarport = super.pystarport.overridePythonAttrs (
      old: {
        buildInputs = (old.buildInputs or [ ]) ++ [ self.poetry ];
      }
    );

    ordered-set = super.ordered-set.overridePythonAttrs (
      old: {
        buildInputs = (old.buildInputs or [ ]) ++ [ self.flit-core ];
      }
    );

  });
}
