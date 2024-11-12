{
  system ? builtins.currentSystem,
  pkgs ? import ./. { inherit system; },
}:
let
  renameExe = pkgs.callPackage ./rename-exe.nix { };
in
pkgs.mkShell {
  buildInputs = [
    pkgs.go-ethereum
    (renameExe pkgs.solc-static-versions.solc_0_6_8 "solc-0.6.8" "solc06")
    (renameExe pkgs.solc-static-versions.solc_0_8_21 "solc-0.8.21" "solc08")
  ];
}
