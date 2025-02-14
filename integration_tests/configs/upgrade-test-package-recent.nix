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
  # release/v1.1.0
  releasedGenesis =
    (fetchFlake "crypto-org-chain/cronos" "526bc803c2f43fd4aadc05a4e16936c3c8e81f29").default;
  # v1.2.2 with skip flush
  released_1 =
    (fetchFlake "mmsqe/cronos" "bd9eaa1cbb535b407061e94488d325263edf0bfa").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = releasedGenesis;
  }
  {
    name = "v1.2";
    path = released_1;
  }
  {
    name = "v1.3";
    path = current;
  }
]
