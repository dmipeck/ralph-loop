{
  description = "ralph-loop: a Ralph Wiggum technique loop runner for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = inputs@{ self, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];

      perSystem = { pkgs, ... }: {
        devShells.default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.gopls pkgs.golangci-lint pkgs.git ];
        };

        packages.ralph-loop = pkgs.buildGoModule {
          pname = "ralph-loop";
          version = "0.1.0";
          src = self;
          vendorHash = "sha256-7K17JaXFsjf163g5PXCb5ng2gYdotnZ2IDKk8KFjNj0=";
          nativeCheckInputs = [ pkgs.git ];
          ldflags = [ "-X github.com/dmipeck/ralph-loop/cmd.version=0.1.0" ];
        };

        packages.claude-plugin = pkgs.runCommand "ralph-loop-claude-plugin" {
          manifest = builtins.toJSON {
            name = "ralph-loop";
            description = "Drive the ralph-loop CLI, which runs the Ralph Wiggum technique against a Claude Code project.";
          };
          passAsFile = [ "manifest" ];
        } ''
          mkdir -p "$out/skills"
          cp -r ${self}/skills/ralph-loop "$out/skills/ralph-loop"

          mkdir -p "$out/.claude-plugin"
          cp "$manifestPath" "$out/.claude-plugin/plugin.json"
        '';
      };

      flake = {
        nixosModules.ralph-loop = { config, lib, pkgs, ... }:
          let cfg = config.programs.ralph-loop;
          in {
            options.programs.ralph-loop = {
              enable = lib.mkEnableOption "ralph-loop, a Ralph Wiggum technique loop runner for Claude Code";
              package = lib.mkOption {
                type = lib.types.package;
                default = self.packages.${pkgs.stdenv.hostPlatform.system}.ralph-loop;
                defaultText = lib.literalExpression "ralph-loop.packages.<system>.ralph-loop";
                description = "The ralph-loop package to install.";
              };
            };

            config = lib.mkIf cfg.enable {
              environment.systemPackages = [ cfg.package ];
            };
          };

        homeModules.ralph-loop = { config, lib, pkgs, ... }:
          let cfg = config.programs.ralph-loop;
          in {
            options.programs.ralph-loop = {
              enable = lib.mkEnableOption "ralph-loop, a Ralph Wiggum technique loop runner for Claude Code";
              package = lib.mkOption {
                type = lib.types.package;
                default = self.packages.${pkgs.stdenv.hostPlatform.system}.ralph-loop;
                defaultText = lib.literalExpression "ralph-loop.packages.<system>.ralph-loop";
                description = "The ralph-loop package to install.";
              };
            };

            config = lib.mkIf cfg.enable {
              home.packages = [ cfg.package ];
            };
          };

        # Legacy alias for consumers still looking under the pre-`homeModules` name.
        homeManagerModules.ralph-loop = self.homeModules.ralph-loop;
      };
    };
}
