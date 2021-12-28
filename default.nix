{ system ? builtins.currentSystem, pkgs ? import ./nix { inherit system; }, db_backend ? "rocksdb" }:
let
  version = "dev";
  pname = "cronosd";
  tags = pkgs.lib.concatStringsSep "," (
    [ "mainnet" ]
    ++ pkgs.lib.lists.optionals (db_backend == "rocksdb") [ "rocksdb" ]
  );
  ldflags = pkgs.lib.concatStringsSep "\n" ([
    "-X github.com/cosmos/cosmos-sdk/version.Name=cronos"
    "-X github.com/cosmos/cosmos-sdk/version.AppName=${pname}"
    "-X github.com/cosmos/cosmos-sdk/version.Version=${version}"
    "-X github.com/cosmos/cosmos-sdk/version.BuildTags=${tags}"
  ] ++ pkgs.lib.lists.optionals (db_backend == "rocksdb") [
    "-X github.com/cosmos/cosmos-sdk/types.DBBackend=rocksdb"
  ]);
  buildInputs = pkgs.lib.lists.optionals (db_backend == "rocksdb") [
    pkgs.rocksdb
  ];
in
pkgs.buildGoApplication rec {
  inherit pname version buildInputs;
  src = (pkgs.nix-gitignore.gitignoreSourcePure [
    "/*" # ignore all, then add whitelists
    "!/x/"
    "!/app/"
    "!/cmd/"
    "!/client/"
    "!go.mod"
    "!go.sum"
    "!gomod2nix.toml"
  ] ./.);
  modules = ./gomod2nix.toml;
  pwd = src; # needed to support replace
  subPackages = [ "cmd/cronosd" ];
  CGO_ENABLED = "1";
  buildFlags = "-tags=${tags}";
  buildFlagsArray = ''
    -ldflags=
    ${ldflags}
  '';
}
