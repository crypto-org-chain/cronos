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
  # Provide threads compatibility for MinGW cross-compilation
  # In newer nixpkgs, targetPackages.threads is not available, but go_1_24 expects it
  threads = super.threads or (if final.stdenv.hostPlatform.isWindows then {
    package = final.windows.pthreads;
  } else null);

  # Override go_1_24 to create go_1_25 with Windows platform support
  # Native nixpkgs go_1_25 doesn't support Windows (x86_64-windows not in meta.platforms)
  # By overriding go_1_24, we inherit its Windows support
  # See: https://github.com/crypto-org-chain/chain-main/pull/1220
  go_1_25 = super.go_1_24.overrideAttrs (old:
    let
      # For MinGW cross-compilation, we need pthreads from the target (Windows) platform
      # The old go_1_24 expects targetPackages.threads.package which doesn't exist in newer nixpkgs
      # So we manually provide it here
      windowsPthreads = if final.stdenv.hostPlatform.isWindows then final.windows.pthreads else null;
    in rec {
    version = "1.25.0";
    src = final.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-S9AekSlyB7+kUOpA1NWpOxtTGl5DhHOyoG4Y4HciciU=";
    };
    # Directly set depsTargetTarget instead of relying on the old value
    # For MinGW targets, we need the pthreads library
    depsTargetTarget = final.lib.optional final.stdenv.targetPlatform.isMinGW windowsPthreads;
    # For Windows cross-compilation, we need to completely avoid the iana-etc patch
    # as it creates a dependency on iana-etc which isn't available for Windows
    # For other platforms, filter out patches that don't apply to Go 1.25
    patches = if final.stdenv.targetPlatform.isWindows then
      # On Windows, use an empty patch list to avoid iana-etc dependency
      []
    else
      # On Unix-like systems, filter patches as before
      builtins.filter (
        patch:
        let
          name = builtins.baseNameOf (builtins.toString patch);
        in
        !(final.lib.hasSuffix "iana-etc-1.17.patch" name)
      ) (old.patches or [ ]);
    # Don't inherit postPatch from go_1_24 as it may contain iana-etc dependencies
    # Go 1.25 source code doesn't need the same patches as Go 1.24
    # If needed, we can add Go 1.25-specific patches here
    postPatch = "";
    # Explicitly add Windows platform support (x86_64-windows, i686-windows)
    # Go 1.24 supports Windows but Go 1.25 upstream doesn't include it in meta.platforms
    meta = old.meta // {
      platforms = (old.meta.platforms or [ ]) ++ final.lib.platforms.windows;
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
