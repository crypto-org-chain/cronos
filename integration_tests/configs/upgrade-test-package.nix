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
  # v0.7.0
  released_0 =
    (fetchFlake "crypto-org-chain/cronos" "799ac47e293403bd57580d2ff96bb8d9851c3cde").default;
  # v0.8.0
  released_1 =
    (fetchFlake "crypto-org-chain/cronos" "2f2cc88b501b47149690fdef05afbbbe5bc116c9").default;
  # v1.0.15
  released_2 =
    (fetchFlake "crypto-org-chain/cronos" "1f5e2618362303d91f621b47cbc1115cf4fa0195").default;
  # v1.1.1
  released_3 =
    (fetchFlake "crypto-org-chain/cronos" "10b8eeb9052e3c52aa59dec15f5d3aca781d1271").default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = released_0;
  }
  {
    name = "v0.7.0-hotfix";
    path = released_1;
  }
  {
    name = "v1.0.0";
    path = released_2;
  }
  {
    name = "v1.1.0";
    path = released_3;
  }
  {
    name = "v1.3";
    path = current;
  }
]
