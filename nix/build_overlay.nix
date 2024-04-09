# some basic overlays nessesary for the build
final: super: {
  rocksdb = final.callPackage ./rocksdb.nix { };

  # make-tarball don't follow symbolic links to avoid duplicate file, the bundle should have no external references.
  # reset the ownership and permissions to make the extract result more normal.
  make-tarball = final.callPackage
    ({ buildPackages
     , runCommand
     }: drv: runCommand "tarball-${drv.name}"
      {
        nativeBuildInputs = with buildPackages; [ gnutar gzip ];
      }
      ''
        tar cfv - -C "${drv}" \
          --owner=0 --group=0 --mode=u+rw,uga+r --hard-dereference . \
          | gzip -9 > $out
      ''
    )
    { };
}
