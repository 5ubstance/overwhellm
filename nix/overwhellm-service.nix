{ config, pkgs, lib, ... }:

with lib;

let
  cfg = config.services.overwhellm;

  overwhellmPkg = pkgs.callPackage ./overwhellm.nix {};
in

{
  options.services.overwhellm = {
    enable = mkEnableOption "overwhellm LLM proxy service";

    port = mkOption {
      type = types.str;
      default = "8080";
      description = "Port the proxy will listen on";
    };

    upstreamUrl = mkOption {
      type = types.str;
      default = "http://aspec.localdomain:12434";
      description = "Upstream LLM server URL";
    };

    timeout = mkOption {
      type = types.int;
      default = 60;
      description = "HTTP client timeout in seconds";
    };

    logLevel = mkOption {
      type = types.enum [ "TRACE" "DEBUG" "INFO" "WARN" "ERROR" "CRITICAL" ];
      default = "INFO";
      description = "Logging level";
    };

    logFile = mkOption {
      type = types.str;
      default = "/var/log/overwhellm/overwhellm.log";
      description = "Path to log file";
    };

    configFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = "Path to config.json file. If null, uses environment variables only.";
    };
  };

  config = mkIf cfg.enable {
    environment.systemPackages = [ overwhellmPkg ];

    users.groups.overwhellm = {};

    users.users.overwhellm = {
      isSystemUser = true;
      group = "overwhellm";
      home = "/var/lib/overwhellm";
      createHome = true;
      description = "overwhellm service user";
    };

    systemd.services.overwhellm = {
      description = "overwhellm LLM Proxy Service";
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        Type = "simple";
        User = "overwhellm";
        Group = "overwhellm";
        WorkingDirectory = "/var/lib/overwhellm";
        Restart = "on-failure";
        RestartSec = 5;
        ExecStart = "${overwhellmPkg}/bin/overwhellm";
        Environment = [
          "PORT=${cfg.port}"
          "UPSTREAM_URL=${cfg.upstreamUrl}"
          "TIMEOUT=${toString cfg.timeout}"
          "LOG_LEVEL=${cfg.logLevel}"
          "LOG_FILE=${cfg.logFile}"
        ];
      };

      preStart = mkIf (cfg.configFile != null) ''
        cp ${cfg.configFile} /var/lib/overwhellm/config.json
        chown overwhellm:overwhellm /var/lib/overwhellm/config.json
      '';
    };

    systemd.tmpfiles.rules = [
      "d /var/log/overwhellm 0755 overwhellm overwhellm -"
      "d /var/lib/overwhellm 0755 overwhellm overwhellm -"
      "L /var/lib/overwhellm/banner - - - - ${overwhellmPkg}/banner"
    ];
  };
}