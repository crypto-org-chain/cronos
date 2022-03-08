{ lib
, buildGoApplication
, nix-gitignore
, rocksdb ? null
, db_backend ? "rocksdb"
, network ? "mainnet"  # mainnet|testnet
}:
let
  version = "dev";
  pname = "cronosd";
  tags = lib.concatStringsSep "," (
    [ network ]
    ++ lib.lists.optionals (db_backend == "rocksdb") [ "rocksdb" ]
  );
  ldflags = lib.concatStringsSep "\n" ([
    "-X github.com/cosmos/cosmos-sdk/version.Name=cronos"
    "-X github.com/cosmos/cosmos-sdk/version.AppName=${pname}"
    "-X github.com/cosmos/cosmos-sdk/version.Version=${version}"
    "-X github.com/cosmos/cosmos-sdk/version.BuildTags=${tags}"
  ] ++ lib.lists.optionals (db_backend == "rocksdb") [
    "-X github.com/cosmos/cosmos-sdk/types.DBBackend=rocksdb"
  ]);
  buildInputs = lib.lists.optionals (db_backend == "rocksdb") [
    rocksdb
  ];
in
buildGoApplication rec {
  inherit pname version buildInputs;
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
