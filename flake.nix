{
  description = "Nix flake for Go + templ + Tailwind build";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";

  outputs = { self, nixpkgs }: let
    system = "x86_64-linux";
    pkgs = import nixpkgs { inherit system; };
    buildInputs = [
      pkgs.go
      pkgs.templ
      pkgs.nodejs_20
      pkgs.nodePackages.tailwindcss
    ];
  in {
    packages.${system}.default = pkgs.stdenv.mkDerivation {
      pname = "transogov2";
      version = "1.0.0";
      src = ./.;

      buildInputs = buildInputs;

      buildPhase = ''
        set -e
        export GOMODCACHE=$TMPDIR/go-mod-cache
        export GOPATH=$TMPDIR/go

        echo "Generating templ files..."
        cd app
        templ generate -path ./views
        cd ..

        echo "Building Tailwind CSS..."
        tailwindcss -i app/static/css/styles.css -o app/static/css/output.css --minify

        echo "Building templ again..."
        templ generate ./app

        echo "Build Go"
        go build -o transogov2 ./app

        echo "Running Go tests"
        cd app && go test -v

        echo "Build done"
      '';

      installPhase = ''
        mkdir -p $out/bin
        cp transogov2 $out/bin/
        # Optionally install CSS as well:
        mkdir -p $out/css
        cp app/static/css/output.css $out/css/
      '';
    };
  };
}

