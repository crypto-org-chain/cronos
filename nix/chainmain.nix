{ pkgs ? import ./default.nix { }, sources ? import ./sources.nix }:
(import sources.chain-main { }).chain-maind

