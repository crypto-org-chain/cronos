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
  # v1.0.15
  releasedGenesis =
    (fetchFlake "crypto-org-chain/cronos" "1f5e2618362303d91f621b47cbc1115cf4fa0195").default;
  # release/v1.1.x
  released1_1 =
    (fetchFlake "crypto-org-chain/cronos" "69a80154b6b24fca15f3562e2c4b312ee1092220").default;
  # release/v1.2.x
  released1_2 =
    (fetchFlake "crypto-org-chain/cronos" "1aea999eef67a0a01b22422bad94b36e45b9759a").default;
  # release/v1.3.x
  released1_3 =
    (fetchFlake "crypto-org-chain/cronos" "dd3cea2df41732ef030a1f830244e340f3cf6bf0").default;
  # release/v1.4.x
  released1_4 =
    (fetchFlake "crypto-org-chain/cronos" "ce797fa995000530ee53cd1fbeb3c67180648002").default;
  released1_4_testnet =
    (fetchFlake "crypto-org-chain/cronos" "2e2f2cd25b16e2a11411b9e2b8f66c6fbea4681c").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = releasedGenesis;
  }
  {
    name = "v1.1.0";
    path = released1_1;
  }
  {
    name = "v1.2";
    path = released1_2;
  }
  {
    name = "v1.3";
    path = released1_3;
  }
  {
    name = "v1.4";
    path = released1_4;
  }
  {
    name = "v1.4.0-rc5-testnet";
    path = released1_4_testnet;
  }
  {
    name = "v1.5";
    path = current;
  }
]
