{
  lib,
  stdenv,
  callPackage,
  buildPackages,
  runCommand,
  bundle-exe,
  rev ? "dirty",
}:
let
  # make-tarball don't follow symbolic links to avoid duplicate file, the bundle should have no external references.
  # reset the ownership and permissions to make the extract result more normal.
  make-tarball =
    drv:
    runCommand "tarball-${drv.name}"
      {
        nativeBuildInputs = with buildPackages; [
          gnutar
          gzip
        ];
      }
      ''
        tar cfv - -C "${drv}" \
          --owner=0 --group=0 --mode=u+rw,uga+r --hard-dereference . \
          | gzip -9 > $out
      '';
  bundle-win-exe = drv: callPackage ./bundle-win-exe.nix { cronosd = drv; };
  matrix = lib.cartesianProductOfSets {
    network = [
      "mainnet"
      "testnet"
    ];
    pkgtype = [
      "nix" # normal nix package
      "bundle" # relocatable bundled package
      "tarball" # tarball of the bundle, for distribution and checksum
    ];
  };
in
builtins.listToAttrs (
  builtins.map (
    { network, pkgtype }:
    {
      name = builtins.concatStringsSep "-" (
        [ "cronosd" ]
        ++ lib.optional (network != "mainnet") network
        ++ lib.optional (pkgtype != "nix") pkgtype
      );
      value =
        let
          cronosd = callPackage ../. {
            inherit rev network;
          };
          bundle = if stdenv.hostPlatform.isWindows then bundle-win-exe cronosd else bundle-exe cronosd;
        in
        if pkgtype == "bundle" then
          bundle
        else if pkgtype == "tarball" then
          make-tarball bundle
        else
          cronosd;
    }
  ) matrix
)
