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
  released = (fetchFlake "crypto-org-chain/cronos" "6ae1f7c448dc8a4d14c334f2df0be4ec0780a53a").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released; }
  { name = "v0.8.0-gravity-alpha3"; path = current; }
]
