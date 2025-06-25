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
  version = "1.15.11";

  src = fetchFromGitHub {
    owner = "ethereum";
    repo = pname;
    rev = "v${version}";
    sha256 = "sha256-2XGKkimwe9h8RxO3SzUta5Bh2Ooldl2LiHqUpn8FK7I=";
  };

  proxyVendor = true;
  vendorHash = "sha256-R9Qg6estiyjMAwN6tvuN9ZuE7+JqjEy+qYOPAg5lIJY=";


  doCheck = false;

  # Move binaries to separate outputs and symlink them back to $out
  postInstall = lib.concatStringsSep "\n" (
    builtins.map (
      bin:
      "mkdir -p \$${bin}/bin && mv $out/bin/${bin} \$${bin}/bin/ && ln -s \$${bin}/bin/${bin} $out/bin/"
    ) bins
  );

  outputs = ["out"] ++ bins;

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
