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
        mockgen = pkgs.callPackage .nix/mockgen.nix {};
        # drpc generator
        protoc-gen-go-drpc = drpc.defaultPackage.${system};

        commonPackages = with pkgs; [
          protoc-gen-go-vtproto
          protoc-gen-go-drpc
          go_1_23
          protobuf
          mockgen
          protoc-gen-go
          pkg-config
          pre-commit
        ];

        devShell = pkgs.mkShell {
          name = "dev shell";
          buildInputs = commonPackages;
          shellHook = ''
                      export GOROOT="${pkgs.go_1_23}/share/go"
                      export PATH="$GOROOT/bin:$PATH"
                    '';
        };

      in
      {
        devShells.default = devShell;
        packages.default = protoc-gen-go-vtproto;
      }
    );
}
