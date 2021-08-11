{ system ? builtins.currentSystem, pkgs ? import ../nix { inherit system; } }:
pkgs.mkShell {
  buildInputs = [
    (import ../. { inherit pkgs; })
    pkgs.go-ethereum
    pkgs.pystarport
    pkgs.start-scripts
    pkgs.poetry
    (pkgs.poetry2nix.mkPoetryEnv { projectDir = ./.; })
  ];
}
