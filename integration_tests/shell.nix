{ system ? builtins.currentSystem, pkgs ? import ../nix { inherit system; } }:
pkgs.mkShell {
  buildInputs = [
    (import ../. { inherit pkgs; })
    pkgs.go-ethereum
    pkgs.pystarport
    pkgs.orchestrator
    pkgs.start-scripts
    pkgs.poetry
    pkgs.yarn
    pkgs.nodejs
    (import ../nix/testenv.nix { inherit pkgs; })
  ];
}
