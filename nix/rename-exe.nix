{ runCommandLocal }:
drv: oldName: newName:
runCommandLocal drv.name { } ''
  mkdir -p $out/bin
  ln -s ${drv}/bin/${oldName} $out/bin/${newName}
''
