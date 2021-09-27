{ system ? builtins.currentSystem, pkgs ? import ./nix { inherit system; } }:
pkgs.buildGoApplication rec {
  pname = "cronosd";
  version = "0.5.4-testnet";
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
  buildFlagsArray = ''
    -ldflags=
    -X github.com/cosmos/cosmos-sdk/version.Name=cronos
    -X github.com/cosmos/cosmos-sdk/version.AppName=${pname}
    -X github.com/cosmos/cosmos-sdk/version.Version=${version}
  '';
}
