{ system ? builtins.currentSystem, pkgs ? import ../nix { inherit system; } }:
pkgs.mkShell {
  buildInputs = [
    pkgs.jq
    pkgs.go
    (import ../. { inherit pkgs; }) # cronosd
    pkgs.start-scripts
    pkgs.go-ethereum
    pkgs.pystarport
    pkgs.orchestrator
    pkgs.poetry
    pkgs.yarn
    pkgs.nodejs
    pkgs.git
    pkgs.dapp
    pkgs.solc-static-versions.solc_0_6_11
    pkgs.test-env
    pkgs.nixpkgs-fmt
  ];
}
