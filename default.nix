{ buildGoModule
, lib
}:

buildGoModule {
  name = "give-me-dns";

  src = ./.;

  vendorHash = "sha256-/8h1rhd3tp4sYjdBI2E+ZrZaEfbeM+6aRae70kXtHQs=";
}
