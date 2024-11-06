{ buildGoModule, fetchFromGitHub }:

let
  version = "1.5.0";
  cosmos-sdk = fetchFromGitHub {
    owner = "cosmos";
    repo = "cosmos-sdk";
    rev = "tools/cosmovisor/v${version}";
    hash = "sha256-Ov8FGpDOcsqmFLT2s/ubjmTXj17sQjBWRAdxlJ6DNEY=";
  };
in
buildGoModule rec {
  name = "cosmovisor";
  version = "1.5.0";
  src = cosmos-sdk + "/tools/cosmovisor";
  subPackages = [ "./cmd/cosmovisor" ];
  vendorHash = "sha256-IkPnnfkofn5w8Oa/uzGxgI1eb5RrJ9haNgj4mBXF+n8=";
  doCheck = false;
}
