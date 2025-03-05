{
  description = "Multi-platform Nix flake (Linux + macOS)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    drpc.url = "github:storj/drpc/v0.0.34";
  };

  outputs =
    {
      self,
      nixpkgs,
      drpc,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        protoc-gen-go-vtproto = pkgs.callPackage .nix/protoc-gen-go-vtproto.nix {};
        # drpc generator
        protoc-gen-go-drpc = drpc.defaultPackage.${system};

        commonPackages = with pkgs; [
          protoc-gen-go-vtproto
          protoc-gen-go-drpc
          go_1_23
          protobuf
          protoc-gen-go
          pkg-config
          pre-commit
        ];

        devShell = pkgs.mkShell {
          name = "dev shell";
          buildInputs = commonPackages;
        };

      in
      {
        devShells.default = devShell;
        packages.default = protoc-gen-go-vtproto;
      }
    );
}
