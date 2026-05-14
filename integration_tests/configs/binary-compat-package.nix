let
  pkgs = import ../../nix { };
  fetchFlake =
    repo: rev:
    (pkgs.flake-compat {
      src = {
        outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
        inherit rev;
        shortRev = builtins.substring 0 7 rev;
      };
    }).defaultNix;

  # Define initialBinary - e.g., previous stable release
  initialBinary = pkgs.callPackage ../../. { }; # current codebase

  # Define newBinary - current/new version
  # This will be the current code being tested
  newBinary = pkgs.callPackage ../../. { }; # current codebase

  # Alternative: test between two specific releases
  # Uncomment and modify as needed:
  # initialBinary = (fetchFlake "crypto-org-chain/cronos" "5cabab487a660e6fbb66c4f9bd5c6eb8228f2b7a").default;
  # newBinary = (fetchFlake "crypto-org-chain/cronos" "1aea999eef67a0a01b22422bad94b36e45b9759a").default;
in
pkgs.runCommand "binary-compat-package" { } ''
   # Create directory structure
   mkdir -p $out/initial/bin
   mkdir -p $out/new/bin

  # Copy binaries (actual copies, not symlinks)
   cp -Lr ${initialBinary}/bin/. $out/initial/bin/
   cp -Lr ${newBinary}/bin/. $out/new/bin/
   
  # Make binaries executable
   find $out/initial/bin $out/new/bin -type f -exec chmod +x {} +
''
