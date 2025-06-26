{
  description = "Development environment for transogov2";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            nodejs_20
            yarn
            docker-compose
            postgresql
            go
            nodePackages.tailwindcss
            nodePackages.postcss
            nodePackages.autoprefixer
          ];

          shellHook = ''
            echo "Dev shell ready with:"
            echo "- Node.js $(node --version)"
            echo "- Yarn $(yarn --version)"
            echo "- Docker Compose $(docker-compose --version)"
            echo "- PostgreSQL $(psql --version)"
            echo "- Go $(go version)"
            echo "- TailwindCSS $(tailwindcss --version)"
          '';
        };
      }
    );
}
