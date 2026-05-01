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
  # builtins.fetchTarball strips the single top-level dir (bin/) so the result
  # contains just `cronosd` at its root.
  # On Linux the binary needs ELF repatching (autoPatchelfHook) to work outside
  # its original Nix closure; on Darwin the Mach-O binary runs as-is.
  fetchRelease =
    tag: version:
    let
      src = builtins.fetchTarball {
        url = "https://github.com/crypto-org-chain/cronos/releases/download/${tag}/cronos_${version}_${arch}.tar.gz";
      };
    in
    pkgs.stdenv.mkDerivation {
      name = "cronos-release-${tag}";
      dontUnpack = true;
      nativeBuildInputs = pkgs.lib.optional pkgs.stdenv.isLinux pkgs.autoPatchelfHook;
      buildInputs = pkgs.lib.optionals pkgs.stdenv.isLinux (
        with pkgs; [ stdenv.cc.cc.lib zlib ]
      );
      installPhase = ''
        mkdir -p $out/bin
        cp ${src}/cronosd $out/bin/cronosd
        chmod +x $out/bin/cronosd
      '';
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
