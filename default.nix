{ buildGoModule
, lib
}:

buildGoModule {
  name = "give-me-dns";

  src = ./.;

  vendorHash = "sha256-N9ZMvFRXisjk0xOixIRKAC+j+zqakwFFlS33fQ/cF1w=";
}
