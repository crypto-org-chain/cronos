{
  lib,
  stdenv,
  buildGoModule,
  fetchFromGitHub,
  libobjc,
  IOKit,
}:

let
  # A list of binaries to put into separate outputs
  bins = [
    "geth"
    "clef"
  ];

in
buildGoModule rec {
  pname = "go-ethereum";
  # Use the old estimateGas implementation
  # https://github.com/crypto-org-chain/go-ethereum/commits/release/1.15-estimateGas/
  version = "1c9b194657000b1593a6162a0d889b801548bf27";

  src = fetchFromGitHub {
    owner = "crypto-org-chain";
    repo = pname;
    rev = version;
    sha256 = "sha256-+uAvVB3uoE7dRfm3hrJOZeuboyeU4RJYD0uUxEYH3X0=";
  };

  proxyVendor = true;
  vendorHash = "sha256-R9Qg6estiyjMAwN6tvuN9ZuE7+JqjEy+qYOPAg5lIJY=";

  doCheck = false;

  outputs = [ "out" ] ++ bins;

  # Move binaries to separate outputs and symlink them back to $out
  postInstall = lib.concatStringsSep "\n" (
    builtins.map (
      bin:
      "mkdir -p \$${bin}/bin && mv $out/bin/${bin} \$${bin}/bin/ && ln -s \$${bin}/bin/${bin} $out/bin/"
    ) bins
  );

  subPackages = [
    "cmd/abidump"
    "cmd/abigen"
    "cmd/blsync"
    "cmd/clef"
    "cmd/devp2p"
    "cmd/era"
    "cmd/ethkey"
    "cmd/evm"
    "cmd/geth"
    "cmd/rlpdump"
    "cmd/utils"
  ];

  # Fix for usb-related segmentation faults on darwin
  propagatedBuildInputs = lib.optionals stdenv.isDarwin [
    libobjc
    IOKit
  ];

  meta = with lib; {
    homepage = "https://geth.ethereum.org/";
    description = "Official golang implementation of the Ethereum protocol";
    license = with licenses; [
      lgpl3Plus
      gpl3Plus
    ];
    maintainers = with maintainers; [
      adisbladis
      lionello
      RaghavSood
    ];
  };
}
