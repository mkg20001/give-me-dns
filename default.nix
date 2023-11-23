{ buildGoModule
, lib
}:

buildGoModule {
  name = "give-me-dns";

  src = ./.;

  vendorHash = "sha256-Zs4bF+OH1YB5DaT0PnFo9g75swT3/Lhllq39TMNxXUQ=";
}
