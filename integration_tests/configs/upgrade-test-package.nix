let
  pkgs = import ../../nix { };
  fetchFlake0 = repo: rev: (pkgs.flake-compat {
    src = {
      outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
      inherit rev;
      shortRev = builtins.substring 0 7 rev;
    };
  }).defaultNix;
  # v1.0.15
  released0 = (fetchFlake0 "crypto-org-chain/cronos" "1f5e2618362303d91f621b47cbc1115cf4fa0195").default;
  fetchFlake = repo: rev: (pkgs.flake-compat {
    src = {
      outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
      inherit rev;
      shortRev = builtins.substring 0 7 rev;
    };
  }).defaultNix;
  # release/v1.1.x
  released = (fetchFlake "crypto-org-chain/cronos" "69a80154b6b24fca15f3562e2c4b312ee1092220").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released0; }
  { name = "v1.1.0"; path = released; }
  { name = "v1.2"; path = current; }
]
