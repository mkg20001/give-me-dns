{
  description = "Tiny script to help you test nixpkgs changes";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:

    let
      supportedSystems = [ "x86_64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f system);
    in

    {
      overlay = final: prev: {
        lxd-nixpkgs-test = prev.callPackage ./. {};
      };

      defaultPackage = forAllSystems (system: (import nixpkgs {
        inherit system;
        overlays = [ self.overlay ];
      }).lxd-nixpkgs-test);

    };
}
