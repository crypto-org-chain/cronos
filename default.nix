{ lib
, buildGoApplication
, nix-gitignore
, go_1_17
, rocksdb ? null
, db_backend ? "rocksdb"
, network ? "mainnet"  # mainnet|testnet
, rev ? "dirty"
}:
let
  version = "v0.7.1";
  pname = "cronosd";
  tags = [ "ledger" "netgo" network ]
    ++ lib.lists.optional (db_backend == "rocksdb") "rocksdb";
  ldflags = lib.concatStringsSep "\n" ([
    "-X github.com/cosmos/cosmos-sdk/version.Name=cronos"
    "-X github.com/cosmos/cosmos-sdk/version.AppName=${pname}"
    "-X github.com/cosmos/cosmos-sdk/version.Version=${version}"
    "-X github.com/cosmos/cosmos-sdk/version.BuildTags=${lib.concatStringsSep "," tags}"
    "-X github.com/cosmos/cosmos-sdk/version.Commit=${rev}"
  ] ++ lib.lists.optionals (db_backend == "rocksdb") [
    "-X github.com/cosmos/cosmos-sdk/types.DBBackend=rocksdb"
  ]);
  buildInputs = lib.lists.optionals (db_backend == "rocksdb") [
    rocksdb
  ];
in
buildGoApplication rec {
  inherit pname version buildInputs tags ldflags;
  src = (nix-gitignore.gitignoreSourcePure [
    "/*" # ignore all, then add whitelists
    "!/x/"
    "!/app/"
    "!/cmd/"
    "!/client/"
    "!go.mod"
    "!go.sum"
    "!gomod2nix.toml"
  ] ./.);
  go = go_1_17;
  modules = ./gomod2nix.toml;
  pwd = src; # needed to support replace
  subPackages = [ "cmd/cronosd" ];
  CGO_ENABLED = "1";

  meta = with lib; {
    description = "Official implementation of the Cronos blockchain protocol";
    homepage = "https://cronos.org/";
    license = licenses.asl20;
    mainProgram = "cronosd";
  };
}
