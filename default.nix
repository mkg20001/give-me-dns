{ buildGoModule
, lib
}:

buildGoModule {
  name = "give-me-dns";

  src = ./.;

  vendorHash = "sha256-C776CpU9xEl2ZKMV/V6PoXxUBdpPlI5g/hTkbsZaNYc=";
}
