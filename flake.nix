{
  description = "hvor - where am I webpage";

  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    utils,
    ...
  }: let
    hvorVersion =
      if (self ? shortRev)
      then self.shortRev
      else "dev";
  in
    {
      overlays.default = _: prev: let
        pkgs = nixpkgs.legacyPackages.${prev.stdenv.hostPlatform.system};
      in {
        hvor = pkgs.callPackage ({buildGo126Module}:
          buildGo126Module {
            pname = "hvor";
            version = hvorVersion;
            src = pkgs.nix-gitignore.gitignoreSource [] ./.;

            patchPhase = ''
              ${pkgs.tailwindcss}/bin/tailwindcss --input ./input.css --output ./static/tailwind.css
            '';

            vendorHash = "sha256-6TbNwlZsftbUv5RjV9E/TMc5cfw6UbpgavMqwz4IOpk=";
          }) {};
      };
    }
    // utils.lib.eachDefaultSystem
    (system: let
      pkgs = import nixpkgs {
        overlays = [self.overlays.default];
        inherit system;
      };
      buildDeps = with pkgs; [
        git
        gnumake
        go_1_26
      ];
      devDeps = with pkgs;
        buildDeps
        ++ [
          golangci-lint
          entr
          tailwindcss
        ];
    in {
      # `nix develop`
      devShells.default = pkgs.mkShell {
        buildInputs = with pkgs;
          [
            (writeShellScriptBin
              "hvorrun"
              ''
                tailwindcss --input ./input.css --output ./static/tailwind.css
                go run . --from-tokens "dev" --verbose
              '')
            (writeShellScriptBin
              "hvordev"
              ''
                ls *.go | entr -r hvorrun
              '')
          ]
          ++ devDeps;
      };

      # `nix build`
      packages = with pkgs; {
        inherit hvor;
        default = hvor;
      };

      # `nix run`
      apps = {
        hvor = utils.lib.mkApp {
          drv = pkgs.hvor;
        };
        default = utils.lib.mkApp {
          drv = pkgs.hvor;
        };
      };
    })
    // {
      nixosModules.default = {
        pkgs,
        lib,
        config,
        ...
      }: let
        cfg = config.services.hvor;
      in {
        options = with lib; {
          services.hvor = {
            enable = mkEnableOption "Enable hvor";

            package = mkOption {
              type = types.package;
              description = ''
                hvor package to use
              '';
              default = pkgs.hvor;
            };

            user = mkOption {
              type = types.str;
              default = "hvor";
              description = "User account under which hvor runs.";
            };

            group = mkOption {
              type = types.str;
              default = "hvor";
              description = "Group account under which hvor runs.";
            };

            dataDir = mkOption {
              type = types.path;
              default = "/var/lib/hvor";
              description = "Path to data dir";
            };

            tailscaleKeyPath = mkOption {
              type = types.path;
            };

            verbose = mkOption {
              type = types.bool;
              default = false;
            };

            localhostPort = mkOption {
              type = types.port;
              default = 56664;
            };

            environmentFile = mkOption {
              type = types.nullOr types.path;
              default = null;
              example = "/var/lib/secrets/hvorSecrets";
            };
          };
        };
        config = lib.mkIf cfg.enable {
          systemd.services.hvor = {
            enable = true;
            script = let
              args =
                [
                  "--ts-key-path ${cfg.tailscaleKeyPath}"
                  "--listen-addr localhost:${toString cfg.localhostPort}"
                ]
                ++ lib.optionals cfg.verbose ["--verbose"];
            in ''
              ${cfg.package}/bin/hvor ${lib.concatStringsSep " " args}
            '';
            wantedBy = ["multi-user.target"];
            after = ["network-online.target"];
            wants = ["network-online.target"];
            serviceConfig = {
              User = cfg.user;
              Group = cfg.group;
              Restart = "always";
              RestartSec = "15";
              WorkingDirectory = cfg.dataDir;
              EnvironmentFile = lib.optional (cfg.environmentFile != null) cfg.environmentFile;
            };
            path = [cfg.package];
          };
        };
      };
    };
}
