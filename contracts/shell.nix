{ system ? builtins.currentSystem, pkgs ? import ../nix { inherit system; } }:
pkgs.mkShell {
  buildInputs = [
    pkgs.jq
    pkgs.dapp
    pkgs.solc-static-versions.solc_0_6_11
  ];
}

