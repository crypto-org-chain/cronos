{ pkgs
, config
, cronos ? (import ../. { inherit pkgs; })
, chainmain ? (import ../nix/chainmain.nix { inherit pkgs; })
, hermes ? (import ../nix/hermes.nix { inherit pkgs; })

}: rec {
  start-chainmain = pkgs.writeShellScriptBin "start-chainmain" ''
    export PATH=${pkgs.test-env}/bin:${chainmain}/bin:$PATH
    ${../scripts/start-chainmain} ${config.chainmain-config} ${config.dotenv} $@
  '';
  start-cronos = pkgs.writeShellScriptBin "start-cronos" ''
    # rely on environment to provide cronosd
    export PATH=${pkgs.test-env}/bin:$PATH
    ${../scripts/start-cronos} ${config.cronos-config} ${config.dotenv} $@
  '';
  start-geth = pkgs.writeShellScriptBin "start-geth" ''
    export PATH=${pkgs.test-env}/bin:${pkgs.go-ethereum}/bin:$PATH
    source ${config.dotenv}
    ${../scripts/start-geth} ${config.geth-genesis} $@
  '';
  start-hermes = pkgs.writeShellScriptBin "start-hermes" ''
    export PATH=${hermes}/bin:$PATH
    source ${config.dotenv}
    ${../scripts/start-hermes} ${config.hermes-config} $@
  '';
  start-scripts = pkgs.symlinkJoin {
    name = "start-scripts";
    paths = [ start-cronos start-geth start-chainmain start-hermes ];
  };
}
