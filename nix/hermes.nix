{
  lib
, stdenv
, fetchurl
, makeWrapper
, openssl
, pkg-config
, darwin
, qemu
}:

let
  version = "1.13.1";
  
  platform = if stdenv.isDarwin then
    if stdenv.isAarch64 then "aarch64-apple-darwin"
    else "x86_64-apple-darwin"
  else if stdenv.isLinux then
    # for linux, always use the x86_64 version
    "x86_64-unknown-linux-gnu"
  else
    throw "Unsupported platform: ${stdenv.system}";
  
  url = "https://github.com/informalsystems/hermes/releases/download/v${version}/hermes-v${version}-${platform}.tar.gz";
  
  sha256 = if platform == "aarch64-apple-darwin" then
    "1j87ikp29008f6x1pcbp8bc77yfhf40sa13d6iliglsisrgsjcas"
  else if platform == "x86_64-apple-darwin" then
    "0f9m8g2xg9l3ghvj42kwa7yn6gr3ralylscmz5bs99qdd5hc8fbd"
  else if platform == "x86_64-unknown-linux-gnu" then
    "0a5anc32brrl390i1aiz3yaar1s9lh3s8r70liw3v7lgd5fnpzgg"
  else
    throw "Unsupported platform: ${stdenv.system}";

in
stdenv.mkDerivation {
  pname = "hermes";
  inherit version;
  
  src = fetchurl {
    inherit url sha256;
  };
  
  nativeBuildInputs = [ makeWrapper ];
  
  buildInputs = lib.optionals stdenv.isDarwin [
    darwin.apple_sdk.frameworks.Security
    darwin.libiconv
    darwin.apple_sdk.frameworks.SystemConfiguration
  ];
  
  sourceRoot = ".";
  
  installPhase = ''
    mkdir -p $out/bin
    cp hermes $out/bin/
    chmod +x $out/bin/hermes
  '';
  
  postFixup = ''
    ${if (stdenv.isLinux && stdenv.isAarch64) then ''
      # for ARM64 uses qemu to simulate x86_64
      mv $out/bin/hermes $out/bin/hermes.x86_64
      cat > $out/bin/hermes << EOF
      #!/bin/sh
      exec ${qemu}/bin/qemu-x86_64 $out/bin/hermes.x86_64 "\$@"
      EOF
      chmod +x $out/bin/hermes
    '' else if stdenv.isLinux then ''
      wrapProgram $out/bin/hermes --prefix LD_LIBRARY_PATH : "${lib.makeLibraryPath [ openssl ]}"
    '' else ''
      wrapProgram $out/bin/hermes
    ''}
  '';
  
  meta = with lib; {
    description = "An IBC Relayer written in Rust";
    homepage = "https://hermes.informal.systems/";
    license = licenses.asl20;
    platforms = platforms.unix;
    maintainers = with maintainers; [ ];
  };

}
