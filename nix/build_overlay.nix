# some basic overlays necessary for the build
final: super:
let
  replaceLast =
    newVal: l:
    let
      len = builtins.length l;
    in
    if len == 0 then [ ] else final.lib.lists.take (len - 1) l ++ [ newVal ];
in
{
  go_1_25 = super.go_1_25.overrideAttrs (old: rec {
    version = "1.25.0";
    src = final.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-S9AekSlyB7+kUOpA1NWpOxtTGl5DhHOyoG4Y4HciciU=";
    };
  });
  rocksdb = final.callPackage ./rocksdb.nix { };
  golangci-lint = final.callPackage ./golangci-lint.nix { };

  # solc-static-versions is broken on Darwin in nixpkgs 25.11 due to legacy SDK usage
  # On Darwin: use pre-built macOS binaries. On Linux: use the original nixpkgs derivations.
  solc-static-versions =
    if final.stdenv.isDarwin then
      {
        solc_0_6_8 = final.stdenv.mkDerivation rec {
          pname = "solc";
          version = "0.6.8";
          src = final.fetchurl {
            url = "https://github.com/ethereum/solidity/releases/download/v${version}/solc-macos";
            sha256 = "sha256-W0qCSwXC9nb1a6pRKPeczT9RZ+1RpVMVCD/vl8xEaEc=";
          };
          dontUnpack = true;
          installPhase = ''
            mkdir -p $out/bin
            cp $src $out/bin/solc-0.6.8
            chmod +x $out/bin/solc-0.6.8
          '';
        };
        solc_0_8_21 = final.stdenv.mkDerivation rec {
          pname = "solc";
          version = "0.8.21";
          src = final.fetchurl {
            url = "https://github.com/ethereum/solidity/releases/download/v${version}/solc-macos";
            sha256 = "sha256-mzXHa30o2kCPW9XhBfr0XsCXS0+7VdPN3bZ4pJG3PU0=";
          };
          dontUnpack = true;
          installPhase = ''
            mkdir -p $out/bin
            cp $src $out/bin/solc-0.8.21
            chmod +x $out/bin/solc-0.8.21
          '';
        };
      }
    else
      super.solc-static-versions;
}
