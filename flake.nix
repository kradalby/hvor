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
      overlay = _: prev: let
        pkgs = nixpkgs.legacyPackages.${prev.system};
      in {
        hvor = pkgs.buildGo121Module {
          pname = "hvor";
          version = hvorVersion;
          src = pkgs.nix-gitignore.gitignoreSource [] ./.;

          patchPhase = ''
            ${pkgs.nodePackages.tailwindcss}/bin/tailwind --input ./input.css --output ./static/tailwind.css
          '';

          vendorSha256 = "sha256-n0pdgGgPWdmfzpz+BuZq+5vlFikLBvqUmNlJRmzjJ2Y=";
        };
      };
    }
    // utils.lib.eachDefaultSystem
    (system: let
      pkgs = import nixpkgs {
        overlays = [self.overlay];
        inherit system;
      };
      buildDeps = with pkgs; [
        git
        gnumake
        go_1_21
      ];
      devDeps = with pkgs;
        buildDeps
        ++ [
          golangci-lint
          entr
          nodePackages.tailwindcss
        ];
    in rec {
      # `nix develop`
      devShell = pkgs.mkShell {
        buildInputs = with pkgs;
          [
            (writeShellScriptBin
              "hvorrun"
              ''
                # if [ ! -f ./static/tailwind.css ]
                # then
                    # echo "static/tailwind.css does not exist, creating..."
                    tailwind --input ./input.css --output ./static/tailwind.css
                # fi
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
      };

      defaultPackage = pkgs.hvor;

      # `nix run`
      apps = {
        hvor = utils.lib.mkApp {
          drv = packages.hvor;
        };
      };

      defaultApp = apps.hvor;

      overlays.default = self.overlay;
    })
    // {
      # TODO(kradalby): Finish the module
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
              ${cfg.package}/bin/hvor ${builtins.concatStringsSep " " args}
            '';
            wantedBy = ["multi-user.target"];
            after = ["network-online.target"];
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
