{ buildGoModule, fetchFromGitHub }:

buildGoModule rec {
  pname = "protoc-gen-go-vtproto";
  version = "latest";

  src = fetchFromGitHub {
    owner = "planetscale";
    repo = "vtprotobuf";
    rev = "ba97887b0a2597d20399eb70221c99c95520e8c1";
    sha256 = "sha256-r4DhjNCZyoxzdJzzh3uNE5LET7xNkUIa2KXYYeuy8PY=";
  };

  subPackages = [ "cmd/protoc-gen-go-vtproto" ];

  vendorHash = "sha256-UMOEePOtOtmm9ShQy5LXcEUTv8/SIG9dU7/9vLhrBxQ=";
}
