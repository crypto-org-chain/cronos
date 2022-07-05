let
  pkgs = import ../../nix { };
  fetchFlake = repo: rev: (pkgs.flake-compat {
    src = {
      outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
      inherit rev;
      shortRev = builtins.substring 0 7 rev;
    };
  }).defaultNix;
  released = (fetchFlake "crypto-org-chain/cronos" "799ac47e293403bd57580d2ff96bb8d9851c3cde").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released; }
  { name = "v0.8.0"; path = current; }
]
