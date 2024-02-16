{ stdenv, fetchurl, lib }:
let
  version = "v1.6.0";
  srcUrl = {
    x86_64-linux = {
      url =
        "https://github.com/informalsystems/hermes/releases/download/${version}/hermes-${version}-x86_64-unknown-linux-gnu.tar.gz";
      sha256 = "sha256-INJ5jiIaa5CVa/0je7Fxtcpf0+E2j7WNT7rD3ACTqts=";
    };
    x86_64-darwin = {
      url =
        "https://github.com/informalsystems/hermes/releases/download/${version}/hermes-${version}-x86_64-apple-darwin.tar.gz";
      sha256 = "sha256-Z1LGmM3PCJFyESqmbkf1rT4ORqtc1JIVzLqb22o8fZ8=";
    };
    aarch64-darwin = {
      url =
        "https://github.com/informalsystems/hermes/releases/download/${version}/hermes-${version}-aarch64-apple-darwin.tar.gz";
      sha256 = "sha256-xzqEu5OlG4jltmTYbO7LaRalSzdGLk7e5T9zUTheKps=";
    };
  }.${stdenv.system} or (throw
    "Unsupported system: ${stdenv.system}");
in
stdenv.mkDerivation {
  name = "hermes";
  inherit version;
  src = fetchurl srcUrl;
  sourceRoot = ".";
  installPhase = ''
    echo "installing hermes ..."
    echo $out
    mkdir -p $out/bin
    install -m 755 -v -D * $out/bin
    echo `env`
  '';
  meta = with lib; { platforms = with platforms; linux ++ darwin; };
}
