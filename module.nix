{ config, pkgs, lib, ... }:

with lib;

let
  cfg = config.services.give-me-dns;
  format = pkgs.formats.yaml {};
in
{
  options = {
    services.give-me-dns = {
      enable = mkEnableOption "give-me-dns";

      settings = mkOption {
        description = "Settings for give-me-dns";
        default = {};
        type = format.type;
      };
    };
  };
  config = lib.mkIf (cfg.enable) {
    services.give-me-dns = {
      settings = mapAttrs' (name: value: nameValuePair (name) (mkDefault value)) {
        store = {
          domain = "6dns.me";
          ttl = "48h";
          file = "/var/lib/give-me-dns/db";
        };
        dns = {
          port = 53;
          ns = [
             "ns1.give-me-dns.net."
             "ns2.give-me-dns.net."
          ];
          mname = "mkg20001.gmail.com.";
        };
        net.port = 9999;
        http.port = 8053;
        provider.wordlist.enable = true;
        provider.random = {
          enable = true;
          id_len = 3;
        };
      };
    };

    systemd.services.give-me-dns = {
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];

      serviceConfig = {
        ExecStart = "${pkgs.give-me-dns}/bin/give-me-dns ${format.generate "config.yaml" cfg.settings}";
        StateDirectory = "give-me-dns";
      };
    };

    networking.firewall.allowedUDPPorts = [
      cfg.settings.dns.port
    ];

    networking.firewall.allowedTCPPorts = [
      cfg.settings.dns.port
      cfg.settings.net.port
    ];
  };
}
