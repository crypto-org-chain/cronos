{ dockerTools, cronos-matrix, testground-testcase }:
let
  patched-cronosd = cronos-matrix.cronosd.overrideAttrs (oldAttrs: {
    patches = oldAttrs.patches or [ ] ++ [
      ./testground-cronosd.patch
    ];
  });
in
dockerTools.buildLayeredImage {
  name = "cronos-testground";
  created = "now";
  contents = [
    testground-testcase
    patched-cronosd
  ];
  config = {
    Expose = [ 9090 26657 26656 1317 26658 26660 26659 30000 ];
    Cmd = [ "/bin/testground-testcase" ];
    Env = [
      "PYTHONUNBUFFERED=1"
    ];
  };
}
