{
  description = "Nix flake for Go 1.24 + templ + Tailwind build";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }: let
    system = "x86_64-linux";
    pkgs = import nixpkgs { inherit system; };

    buildInputs = [
      pkgs.go_1_24
      pkgs.templ
      pkgs.nodejs_20
      pkgs.nodePackages.tailwindcss
    ];
  in {
    packages.${system}.default = pkgs.stdenv.mkDerivation {
      pname = "transogov2";
      version = "1.0.0";
      src = ./.;
      vendorSha256 = "1phj98gwhanilxjp6n1fiiqvakwnjfxmsz4vhcp6md7vd49g425b";

      buildInputs = buildInputs;

      buildPhase = ''
        set -e
        export GOMODCACHE=$TMPDIR/go-mod-cache
        export GOPATH=$TMPDIR/go
        export GOTOOLCHAIN=local
        unset HOME

        echo "Generating templ files..."
        cd app
        templ generate -path ./views
        cd ..

        echo "Building Tailwind CSS..."
        tailwindcss -i app/static/css/styles.css -o app/static/css/output.css --minify

        echo "Building templ again..."
        templ generate ./app

        echo "Build Go"
        go build -mod=vendor -o transogov2 ./app

        echo "Running Go tests"
        cd app && go test -mod=vendor -v

        echo "Build done"
      '';

      installPhase = ''
        mkdir -p $out/bin
        cp transogov2 $out/bin/
        mkdir -p $out/css
        cp app/static/css/output.css $out/css/
      '';
    };
  };
}
