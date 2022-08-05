let
  pkgs = import ../../nix { };
  fetchFlake = repo: rev: (pkgs.flake-compat {
    src = {
      outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
      inherit rev;
      shortRev = builtins.substring 0 7 rev;
    };
  }).defaultNix;
  released = (fetchFlake "crypto-org-chain/cronos" "8035ebdcc3ce1ed5ee3d4887eb4ea1032c4d283c").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released; }
  { name = "v0.7.0-hotfix"; path = current; }
]
