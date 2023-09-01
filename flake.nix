{
  description = "Temporary DNS names for IPv6 addresses";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:

    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f system);
    in

    {
      overlays.default = final: prev: {
        give-me-dns = prev.callPackage ./. {};
      };

      packages = forAllSystems (system:
        let
          pkgs = (import nixpkgs {
            inherit system;
            overlays = [ self.overlays.default ];
          });
        in
          {
            inherit (pkgs) give-me-dns;

            default = pkgs.give-me-dns;
          });

      nixosModules = {
        give-me-dns = import ./module.nix;
      };
    };
}
