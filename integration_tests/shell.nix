{
  system ? builtins.currentSystem,
  pkgs ? import ../nix { inherit system; },
}:
pkgs.mkShell {
  buildInputs = [
    (pkgs.callPackage ../. { coverage = true; }) # cronosd
    pkgs.start-scripts
    pkgs.go-ethereum
    pkgs.go
    pkgs.cosmovisor
    pkgs.nodejs
    pkgs.test-env
    pkgs.chain-maind
    pkgs.hermes
    pkgs.rly
  ];
  shellHook = ''
    mkdir -p ./coverage
    export GOCOVERDIR=./coverage
    export TMPDIR=/tmp
  '';
}
