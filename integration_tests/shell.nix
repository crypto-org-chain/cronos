{
  system ? builtins.currentSystem,
  pkgs ? import ../nix { inherit system; },
}:
pkgs.mkShell {
  buildInputs = [
    (pkgs.callPackage ../. { coverage = true; }) # cronosd
    pkgs.start-scripts
    pkgs.go-ethereum
    pkgs.cosmovisor
    pkgs.nodejs
    pkgs.test-env
    pkgs.chain-maind
    pkgs.hermes
    pkgs.rly
  ];
  shellHook = ''
    mkdir ./coverage
    export GOCOVERDIR=./coverage
  '';
}
