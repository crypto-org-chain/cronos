{ lib
, buildGoApplication
, nix-gitignore
, rocksdb ? null
, network ? "mainnet"  # mainnet|testnet
, rev ? "dirty"
}:
let
  version = "v0.9.0";
  pname = "cronosd";
  tags = [ "ledger" "netgo" network "mdbx" ]
    ++ lib.lists.optionals (rocksdb != null) [ "rocksdb" "rocksdb_build" ];
  ldflags = lib.concatStringsSep "\n" ([
    "-X github.com/cosmos/cosmos-sdk/version.Name=cronos"
    "-X github.com/cosmos/cosmos-sdk/version.AppName=${pname}"
    "-X github.com/cosmos/cosmos-sdk/version.Version=${version}"
    "-X github.com/cosmos/cosmos-sdk/version.BuildTags=${lib.concatStringsSep "," tags}"
    "-X github.com/cosmos/cosmos-sdk/version.Commit=${rev}"
  ]);
  buildInputs = lib.lists.optional (rocksdb != null) rocksdb;
in
buildGoApplication rec {
  inherit pname version buildInputs tags ldflags;
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

  meta = with lib; {
    description = "Official implementation of the Cronos blockchain protocol";
    homepage = "https://cronos.org/";
    license = licenses.asl20;
    mainProgram = "cronosd";
  };
}
