{
  system ? builtins.currentSystem,
  pkgs ? import ../nix { inherit system; },
}:
pkgs.mkShell {
  buildInputs = [
    pkgs.which
    pkgs.jq
    pkgs.dapp
    pkgs.solc-versions.solc_0_6_8
  ];
}
