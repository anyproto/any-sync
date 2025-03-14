{ buildGoModule, fetchFromGitHub }:

buildGoModule rec {
  pname = "mockgen";
  version = "latest";

  src = fetchFromGitHub {
    owner = "uber-go";
    repo = "mock";
    rev = "main";
    sha256 = "sha256-I/gy0rXL0DWcfXrkAx21a2xIDaj6w3wrrO7+z8HHMo0=";
  };

  subPackages = [ "mockgen" ];

  vendorHash = "sha256-0OnK5/e0juEYrNJuVkr+tK66btRW/oaHpJSDakB32Bc=";
}
