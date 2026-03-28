(import <nixpkgs> {
  overlays = [
    (self: super: {
      overwhellm = self.callPackage ./nix/overwhellm.nix {};
    })
  ];
}).overwhellm