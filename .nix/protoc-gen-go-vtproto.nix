{ buildGoModule, fetchFromGitHub }:

buildGoModule rec {
  pname = "protoc-gen-go-vtproto";
  version = "latest";

  src = fetchFromGitHub {
    owner = "planetscale";
    repo = "vtprotobuf";
    rev = "main";  # Последний коммит
    sha256 = "sha256-zxR1yye/6ldN234FOplLbZ/weaohAiM8JL/KxunXsnk=";
  };

  subPackages = [ "cmd/protoc-gen-go-vtproto" ];

  vendorHash = "sha256-UMOEePOtOtmm9ShQy5LXcEUTv8/SIG9dU7/9vLhrBxQ=";
}
