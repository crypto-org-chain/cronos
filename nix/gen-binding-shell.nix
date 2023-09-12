{ system ? builtins.currentSystem, pkgs ? import ../nix { inherit system; } }:
let
  renameExe = pkgs.callPackage ../nix/rename-exe.nix { };
in
pkgs.mkShell {
  buildInputs = [
    pkgs.go-ethereum
    (renameExe pkgs.solc-static-versions.solc_0_8_21 "solc-0.8.21" "solc08")
  ];
}
