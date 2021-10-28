{ pkgs ? import ./default.nix { } }:
pkgs.stdenv.mkDerivation {
  name = "hermes";
  version = "v0.7.3";
  src = pkgs.fetchurl {
    url =
      "https://github.com/informalsystems/ibc-rs/releases/download/v0.7.1/hermes-v0.7.1-x86_64-unknown-linux-gnu.tar.gz";
    sha256 = "sha256:1zjirchann6q1nszxkb09wrkf21di09qdni9q9v4mg75xxc9i7h3";
  };
  sourceRoot = ".";
  installPhase = ''
    echo "hermes"
    echo $out
    install -m755 -D hermes $out/bin/hermes
  '';

  meta = with pkgs.lib; { platforms = platforms.linux; };

}
