{ lib
, stdenv
, buildGoApplication
, nix-gitignore
, buildPackages
, rocksdb
, network ? "mainnet"  # mainnet|testnet
, rev ? "dirty"
}:
let
  version = "v1.0.5";
  pname = "cronosd";
  tags = [ "ledger" "netgo" network "rocksdb" "grocksdb_no_link" ];
  ldflags = lib.concatStringsSep "\n" ([
    "-X github.com/cosmos/cosmos-sdk/version.Name=cronos"
    "-X github.com/cosmos/cosmos-sdk/version.AppName=${pname}"
    "-X github.com/cosmos/cosmos-sdk/version.Version=${version}"
    "-X github.com/cosmos/cosmos-sdk/version.BuildTags=${lib.concatStringsSep "," tags}"
    "-X github.com/cosmos/cosmos-sdk/version.Commit=${rev}"
  ]);
  buildInputs = [ rocksdb ];
in
buildGoApplication rec {
  inherit pname version buildInputs tags ldflags;
  # specify explicitly to workaround issue: https://github.com/nix-community/gomod2nix/issues/106
  go = buildPackages.go_1_19;
  src = (nix-gitignore.gitignoreSourcePure [
    "/*" # ignore all, then add whitelists
    "!/x/"
    "!/app/"
    "!/cmd/"
    "!/client/"
    "!/versiondb/"
    "!go.mod"
    "!go.sum"
    "!gomod2nix.toml"
  ] ./.);
  modules = ./gomod2nix.toml;
  pwd = src; # needed to support replace
  subPackages = [ "cmd/cronosd" ];
  CGO_ENABLED = "1";
  CGO_LDFLAGS =
    if stdenv.hostPlatform.isWindows
    then "-lrocksdb-shared"
    else "-lrocksdb -pthread -lstdc++ -ldl";

  postFixup = lib.optionalString stdenv.isDarwin ''
    ${stdenv.cc.targetPrefix}install_name_tool -change "@rpath/librocksdb.7.dylib" "${rocksdb}/lib/librocksdb.dylib" $out/bin/cronosd
  '';

  doCheck = false;
  meta = with lib; {
    description = "Official implementation of the Cronos blockchain protocol";
    homepage = "https://cronos.org/";
    license = licenses.asl20;
    mainProgram = "cronosd" + stdenv.hostPlatform.extensions.executable;
    platforms = platforms.all;
  };
}
