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
  # v1.1.1, https://github.com/crypto-org-chain/cronos/pull/1382
  released = (fetchFlake "yihuang/cronos" "88a1d987c48a884751c2f6d5584c766624592752").default;
  current = pkgs.callPackage ../../. { };
  farm = pkgs.linkFarm "upgrade-test-package" [
    { name = "genesis/bin"; path = "${released0}/bin"; }
    { name = "v1.1.0/bin"; path = "${released}/bin"; }
    { name = "v1.2/bin"; path = "${current}/bin"; }
  ];
in
pkgs.make-tarball farm
