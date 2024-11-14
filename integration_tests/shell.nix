{
  system ? builtins.currentSystem,
  pkgs ? import ../nix { inherit system; },
}:
let
  renameExe = pkgs.callPackage ../nix/rename-exe.nix { };
in
pkgs.mkShell {
  buildInputs = [
    pkgs.jq
    pkgs.go
    pkgs.gomod2nix
    (pkgs.callPackage ../. { coverage = true; }) # cronosd
    pkgs.start-scripts
    pkgs.go-ethereum
    pkgs.cosmovisor
    pkgs.poetry
    pkgs.nodejs
    pkgs.git
    pkgs.dapp
    (renameExe pkgs.solc-static-versions.solc_0_6_8 "solc-0.6.8" "solc06")
    (renameExe pkgs.solc-static-versions.solc_0_8_21 "solc-0.8.21" "solc08")
    pkgs.test-env
    pkgs.nixfmt-rfc-style
    pkgs.rocksdb
    pkgs.chain-maind
    pkgs.hermes
    pkgs.rly
  ];
  shellHook = ''
    mkdir ./coverage
    export GOCOVERDIR=./coverage
  '';
}
