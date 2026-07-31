let
  pkgs = import ../../nix { };
  fetchFlake =
    repo: rev: sha256:
    (pkgs.flake-compat {
      src = {
        outPath = builtins.fetchTarball {
          url = "https://github.com/${repo}/archive/${rev}.tar.gz";
          inherit sha256;
        };
        inherit rev;
        shortRev = builtins.substring 0 7 rev;
      };
    }).defaultNix;
  # v1.7.8-unsafe tag, rev d4c278518a7a14723d936e6abaf9d63b15de7520.
  # sha256 is a placeholder (lib.fakeSha256) - nix reports the real hash on
  # first build failure; fill it in from that error before merging.
  releasedV178Unsafe =
    (fetchFlake "crypto-org-chain/cronos" "d4c278518a7a14723d936e6abaf9d63b15de7520"
      pkgs.lib.fakeSha256
    ).default;
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-rehearsal-package" [
  {
    name = "genesis";
    path = releasedV178Unsafe;
  }
  {
    name = "v1.8";
    path = current;
  }
]
