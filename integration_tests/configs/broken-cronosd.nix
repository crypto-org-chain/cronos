{
  pkgs ? import ../../nix { },
}:
let
  cronosd = (pkgs.callPackage ../../. { });
in
cronosd.overrideAttrs (oldAttrs: {
  patches = oldAttrs.patches or [ ] ++ [
    ./broken-cronosd.patch
  ];
})
