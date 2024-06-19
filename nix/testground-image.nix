{ dockerTools, cronos-matrix, test-env, testground-testcase }:
dockerTools.buildLayeredImage {
  name = "cronos-testground";
  contents = [
    testground-testcase
    test-env
    cronos-matrix.cronosd
  ];
  config = {
    Expose = [ 9090 26657 26656 1317 26658 26660 26659 30000 ];
    Cmd = [ "/bin/testground-testcase" ];
    Env = [
      "PYTHONUNBUFFERED=1"
    ];
  };
}
