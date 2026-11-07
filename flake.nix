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
        packages.default = pkgs.callPackage ./nix/package.nix { };
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
    )
    // {
      nixosModules.default = import ./nix/module.nix { inherit self; };
    };
}
