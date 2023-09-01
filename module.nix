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
        domain = "6dns.me";
        ttl = "48h";
        idlen = 3;
        dns_addr = "::";
        dns_port = 53;
        net_addr = "::";
        net_port = 9999;
        store_file = "/var/lib/give-me-dns/db";
        dns_ns = "ns1.give-me-dns.net";
        dns_mname = "mkg20001.gmail.com.";
      };
    };

    systemd.services.give-me-dns = {
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];

      serviceConfig = {
        ExecStart = "${pkgs.give-me-dns}/bin/give-me-dns ${format.generate "config.yaml" cfg}";
        StateDirectory = "give-me-dns";
      };
    };

    networking.firewall.allowedUDPPorts = [
      cfg.settings.dns_port
    ];

    networking.firewall.allowedTCPPorts = [
      cfg.settings.net_port
    ];
  };
}
