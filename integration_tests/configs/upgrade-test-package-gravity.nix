let
  pkgs = import ../../nix { };
  fetchFlake = repo: rev: (pkgs.flake-compat {
    src = {
      outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
      inherit rev;
      shortRev = builtins.substring 0 7 rev;
    };
  }).defaultNix;
  # tag: v0.8.0-gravity-alpha2
  released = (fetchFlake "crypto-org-chain/cronos" "57260c7c21cdedffd75480e8cb4e8838ea6a16b5").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released; }
  { name = "v0.8.0-gravity-alpha3"; path = current; }
]
