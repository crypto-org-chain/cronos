{ dapptools-release, dapptools-master }:
self: super:
let
  dapptools =
    {
      x86_64-linux = (import (dapptools-release + "/release.nix") { }).dapphub.linux.stable;
      x86_64-darwin = (import (dapptools-release + "/release.nix") { }).dapphub.darwin.stable;
    }
    .${self.system} or (throw "Unsupported system: ${self.system}");
  dapptools-patched = self.srcOnly {
    name = "dapptools-patched";
    src = dapptools-master;
    patches = [
      ./dapptools.patch
    ];
  };
in
{
  # use released version to hit the binary cache
  inherit (dapptools) dapp solc-versions;
  # use the patched version to access solc-static-versions
  inherit (import (dapptools-patched + "/overlay.nix") self super) solc-static-versions;
}
