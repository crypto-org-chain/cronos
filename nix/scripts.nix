{ pkgs
, config
, cronos ? (import ../. { inherit pkgs; })
}:
rec {
  start-cronos = pkgs.writeShellScriptBin "start-cronos" ''
    export PATH=${pkgs.pystarport}/bin:${cronos}/bin:$PATH
    ${../scripts/start-cronos} ${config.cronos-config} $@
  '';
  start-geth = pkgs.writeShellScriptBin "start-geth" ''
    export PATH=${pkgs.go-ethereum}/bin:$PATH
    ${../scripts/start-geth} ${config.geth-genesis} $@
  '';
  start-scripts = pkgs.symlinkJoin {
    name = "start-scripts";
    paths = [ start-cronos start-geth ];
  };
}
