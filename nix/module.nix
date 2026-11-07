{ self }:
{
  config,
  lib,
  pkgs,
  utils,
  ...
}:
let
  cfg = config.services.keeplink-exporter;
  yaml = pkgs.formats.yaml { };
  switches = lib.imap0 (
    index: switch:
    switch
    // {
      credentialPrefix = "switch-${toString index}";
    }
  ) cfg.config.switches;
  credentialsDirectory = "/run/credentials/keeplink-exporter.service";
  configFile = yaml.generate "keeplink-exporter.yaml" {
    switches = map (switch: {
      switch_ip = switch.switchIP;
      username_file = "${credentialsDirectory}/${switch.credentialPrefix}-username";
      password_file = "${credentialsDirectory}/${switch.credentialPrefix}-password";
      inherit (switch) timeout;
    }) switches;
  };
in
{
  options.services.keeplink-exporter = {
    enable = lib.mkEnableOption "Keeplink Prometheus exporter";
    listenAddress = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1:8192";
      example = "0.0.0.0:8192";
      description = "Address and port on which to serve Prometheus metrics.";
    };
    config.switches = lib.mkOption {
      type = lib.types.listOf (
        lib.types.submodule {
          options = {
            switchIP = lib.mkOption {
              type = lib.types.str;
              example = "192.168.1.168";
              description = "Address of the switch to scrape.";
            };
            usernameFile = lib.mkOption {
              type = lib.types.strMatching "/.*";
              description = "Absolute runtime path to the username file, loaded as a systemd credential.";
            };
            passwordFile = lib.mkOption {
              type = lib.types.strMatching "/.*";
              description = "Absolute runtime path to the password file, loaded as a systemd credential.";
            };
            timeout = lib.mkOption {
              type = lib.types.str;
              default = "0s";
              example = "10s";
              description = "HTTP request timeout as a Go duration string; zero disables the timeout.";
            };
          };
        }
      );
      default = [ ];
      description = "Switches to include in the generated scrape configuration.";
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = cfg.config.switches != [ ];
        message = "services.keeplink-exporter.config.switches must contain at least one switch.";
      }
    ];

    systemd.services.keeplink-exporter = {
      description = "Keeplink Prometheus exporter";
      wantedBy = [ "multi-user.target" ];
      wants = [ "network-online.target" ];
      after = [ "network-online.target" ];
      serviceConfig = {
        ExecStart = utils.escapeSystemdExecArgs [
          (lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.default)
          "--scrape-config"
          configFile
          "--listen"
          cfg.listenAddress
        ];
        DynamicUser = true;
        LoadCredential = lib.concatMap (switch: [
          "${switch.credentialPrefix}-username:${switch.usernameFile}"
          "${switch.credentialPrefix}-password:${switch.passwordFile}"
        ]) switches;
        Restart = "on-failure";
        RestartSec = 5;

        CapabilityBoundingSet = "";
        NoNewPrivileges = true;
        UMask = "0077";

        PrivateDevices = true;
        PrivateTmp = true;
        ProtectHome = true;
        ProtectSystem = "strict";
        ProtectProc = "invisible";

        ProtectClock = true;
        ProtectControlGroups = true;
        ProtectHostname = true;
        ProtectKernelLogs = true;
        ProtectKernelModules = true;
        ProtectKernelTunables = true;

        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        RestrictNamespaces = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        RestrictAddressFamilies = [
          "AF_UNIX"
          "AF_INET"
          "AF_INET6"
          # Used by libc for local interface discovery during DNS resolution.
          "AF_NETLINK"
        ];
        SystemCallArchitectures = "native";
        SystemCallFilter = [ "@system-service" ];
        SystemCallErrorNumber = "EPERM";
      };
    };
  };
}
