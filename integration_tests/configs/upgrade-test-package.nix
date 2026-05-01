let
  pkgs = import ../../nix { };
  # Map Nix system tuple → cronos release arch string
  arch =
    {
      "x86_64-linux" = "Linux_x86_64";
      "aarch64-linux" = "Linux_arm64";
      "x86_64-darwin" = "Darwin_x86_64";
      "aarch64-darwin" = "Darwin_arm64";
    }
    .${pkgs.stdenv.system} or (throw "unsupported system: ${pkgs.stdenv.system}");
  # Download a pre-built release binary instead of compiling from source.
  # The release tarballs have structure ./bin/cronosd, so fetchTarball strips
  # the leading ./ and the result already contains bin/cronosd at its root.
  fetchRelease =
    tag: version:
    builtins.fetchTarball {
      url = "https://github.com/crypto-org-chain/cronos/releases/download/${tag}/cronos_${version}_${arch}.tar.gz";
    };
  current = pkgs.callPackage ../../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = fetchRelease "v1.0.15" "1.0.15";
  }
  {
    name = "v1.1.0";
    path = fetchRelease "v1.1.1" "1.1.1";
  }
  {
    name = "v1.2";
    path = fetchRelease "v1.2.0" "1.2.0";
  }
  {
    name = "v1.3";
    path = fetchRelease "v1.3.4" "1.3.4";
  }
  {
    name = "v1.4";
    path = fetchRelease "v1.4.8" "1.4.8";
  }
  {
    name = "v1.5";
    path = fetchRelease "v1.5.4" "1.5.4";
  }
  {
    name = "v1.6";
    path = fetchRelease "v1.6.1" "1.6.1";
  }
  {
    name = "v1.7";
    path = fetchRelease "v1.7.0" "1.7.0";
  }
  {
    name = "v1.8";
    path = current;
  }
]
