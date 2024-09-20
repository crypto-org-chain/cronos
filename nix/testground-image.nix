{ dockerTools, runCommandLocal, cronos-matrix, benchmark-testcase }:
let
  patched-cronosd = cronos-matrix.cronosd.overrideAttrs (oldAttrs: {
    patches = oldAttrs.patches or [ ] ++ [
      ./testground-cronosd.patch
    ];
  });
in
let
  tmpDir = runCommandLocal "tmp" { } ''
    mkdir -p $out/tmp/
  '';
in
dockerTools.buildLayeredImage {
  name = "cronos-testground";
  created = "now";
  contents = [
    benchmark-testcase
    patched-cronosd
    tmpDir
  ];
  config = {
    Expose = [ 9090 26657 26656 1317 26658 26660 26659 30000 ];
    Cmd = [ "/bin/stateless-testcase" ];
    Env = [
      "PYTHONUNBUFFERED=1"
    ];
  };
}
