{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-26.05";
    utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.callPackage (
          { buildGoModule }:
          buildGoModule {
            pname = "keeplink-exporter";
            version = "1.0.0";

            src = ./.;
            vendorHash = null;

            buildPhase = ''
              runHook preBuild
              make build
              runHook postBuild
            '';

            installPhase = ''
              runHook preInstall
              mkdir -p $out/bin
              cp bin/exporter-keeplink $out/bin
              runHook postInstall
            '';

            meta = {
              description = "Prometheus exporter for Keeplink KP-9000-9XHML-X";
              homepage = "https://git.cobalt.rocks/cobalt/keeplink-exporter";
              maintainers = [ nixpkgs.lib.maintainers.cobalt ];
              mainProgram = "exporter-keeplink";
              platforms = [
                "aarch64-linux"
                "x86_64-linux"
              ];
            };
          }
        ) { };
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # golang linter and vuln checker
            gotools
            gosec

            # golang build tooling
            gcc
            pkg-config
            go
            gnumake
            findutils
            coreutils
          ];
        };
      }
    );
}
