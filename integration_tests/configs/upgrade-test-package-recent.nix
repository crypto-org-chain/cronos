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
  # release/v1.3.x
  releasedGenesis =
    (fetchFlake "crypto-org-chain/cronos" "e1d819c862b30f0ce978baf2addb12516568639e").default;
  # release/v1.4.x
  released1_4 =
    (fetchFlake "crypto-org-chain/cronos" "ce797fa995000530ee53cd1fbeb3c67180648002").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = releasedGenesis;
  }
  {
    name = "v1.4";
    path = released1_4;
  }
  {
    name = "v1.4.0-rc5-testnet";
    path = current;
  }
]
