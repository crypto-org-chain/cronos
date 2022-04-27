let
  pkgs = import ../../nix { };
  released = import (builtins.fetchTarball "https://github.com/crypto-org-chain/cronos/archive/v0.6.5.tar.gz") { };
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released; }
  { name = "v0.7.0"; path = current; }
]
