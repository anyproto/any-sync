{ pkgs ? import <nixpkgs> {} }:
pkgs.buildGoModule rec {
  pname = "protoc-gen-gogofaster";
  name = pname;
  src = pkgs.fetchFromGitHub {
    owner = "anyproto";
    repo = "protobuf";
    rev = "marshal-append";
    hash = "sha256-TetJNcPn0/2Dym/JOLczGax4BEnsLKvJtTZIv1mTMdc=";
  };

  vendorHash = "sha256-nOL2Ulo9VlOHAqJgZuHl7fGjz/WFAaWPdemplbQWcak=";

  doCheck = false;
}
