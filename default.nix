{ buildGoModule
, lib
}:

buildGoModule {
  name = "give-me-dns";

  src = ./.;

  vendorHash = "sha256-YB9rGnjnFkhbpFcjJnfrEH7Keups4mdRyHIs1clN9QI=";
}
