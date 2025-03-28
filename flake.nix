{
  description = "";
  inputs.nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1.0.tar.gz";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.drpc.url = "github:storj/drpc/v0.0.34";

  outputs = { self, nixpkgs, drpc, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
        config = { allowUnfree = true; };
      };

      # any-protoc-gen-gogofaster = pkgs.callPackage ./nix/any-protoc-gen-gogofaster.nix {};
      # protoc-gen-go-drpc = drpc.defaultPackage.${system};
    in {
      devShell = pkgs.mkShell {
        name = "any-sync";
        nativeBuildInputs = [
          # our forked proto generator
          # any-protoc-gen-gogofaster

          # drpc generator
          # protoc-gen-go-drpc

          pkgs.go_1_23
          # pkgs.gox
          pkgs.protobuf3_21
          pkgs.pkg-config
          pkgs.pre-commit
          # todo: govvv, not packaged
        ];
      };
    });
}
