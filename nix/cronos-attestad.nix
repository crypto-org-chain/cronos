{ buildGo123Module, lib, stdenv }:

# For now, use a wrapper that points to the locally built binary
# Once the cronos-attestation-layer repo is public on GitHub,
# this will be replaced with a proper buildGoModule derivation

stdenv.mkDerivation rec {
  pname = "cronos-attestad";
  version = "0.1.0-local";

  src = builtins.path {
    path = /Users/randy.ang/Documents/code/cronos-attestation-layer;
    name = "cronos-attestation-layer-src";
  };

  phases = [ "installPhase" ];

  installPhase = ''
    mkdir -p $out/bin
    if [ -f ${src}/build/cronos-attestad ]; then
      cp ${src}/build/cronos-attestad $out/bin/
    else
      echo "cronos-attestad not found in ${src}/build/"
      echo "Please run 'make build' in the cronos-attestation-layer directory"
      echo "Creating a placeholder that will fail if executed"
      cat > $out/bin/cronos-attestad << 'EOF'
#!/bin/sh
echo "ERROR: cronos-attestad not built yet!"
echo "Please run: cd /Users/jaytseng/workspace/cronos-attestation-layer && make build"
exit 1
EOF
      chmod +x $out/bin/cronos-attestad
    fi
  '';

  meta = with lib; {
    description = "Cronos Attestation Layer - L2 attestation service (local development)";
    homepage = "https://github.com/crypto-org-chain/cronos-attestation-layer";
    license = licenses.asl20;
    maintainers = with maintainers; [ ];
    platforms = platforms.unix;
  };
}

