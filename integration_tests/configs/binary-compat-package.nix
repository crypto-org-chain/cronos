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
  
  # Define binary1 - e.g., previous stable release
  # Change this to the version you want to test against
  binary1Source = pkgs.callPackage ../../. { };

  
  # Define binary2 - current/new version
  # This will be the current code being tested
  binary2Source = pkgs.callPackage ../../. { };
  
  # Alternative: test between two specific releases
  # Uncomment and modify as needed:
  # binary1Source = (fetchFlake "crypto-org-chain/cronos" "5cabab487a660e6fbb66c4f9bd5c6eb8228f2b7a").default; # v1.1.x
  # binary2Source = (fetchFlake "crypto-org-chain/cronos" "1aea999eef67a0a01b22422bad94b36e45b9759a").default; # v1.2.x
  
  # Test expectation: set to true if you expect a breaking change (app hash mismatch)
  # set to false if you expect non-breaking (continued block production)
  expect_breaking = false;
in
pkgs.runCommand "binary-compat-package" {} ''
  # Create directory structure
  mkdir -p $out/binary1/bin
  mkdir -p $out/binary2/bin
  
  # Copy binaries (actual copies, not symlinks)
  cp -r ${binary1Source}/bin/* $out/binary1/bin/
  cp -r ${binary2Source}/bin/* $out/binary2/bin/
  
  # Make binaries executable
  chmod +x $out/binary1/bin/*
  chmod +x $out/binary2/bin/*
  
  # Create config file
  cat > $out/config.json <<EOF
  ${builtins.toJSON { inherit expect_breaking; }}
  EOF
''

