{
  description = "ralph-loop: a Ralph Wiggum technique loop runner for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = inputs@{ self, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];

      perSystem = { pkgs, config, ... }: {
        devShells.default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.gopls pkgs.golangci-lint pkgs.git ];
        };

        packages.ralph-loop = pkgs.buildGoModule {
          pname = "ralph-loop";
          version = "0.5.0";
          src = self;
          vendorHash = "sha256-7K17JaXFsjf163g5PXCb5ng2gYdotnZ2IDKk8KFjNj0=";
          nativeCheckInputs = [ pkgs.git ];
          ldflags = [ "-X github.com/dmipeck/ralph-loop/cmd.version=0.5.0" ];
        };

        # A complete, installable `ralph` plugin bundle: the repo's checked-in
        # plugin directories (.claude-plugin/, skills/, commands/, agents/,
        # hooks/, scripts/ — those are the single source of truth, nothing
        # regenerated here) plus a prebuilt `ralph-loop` binary at bin/, so
        # scripts/ralph-lib.sh's `ralph_find_binary` finds it without needing
        # a Go toolchain at plugin-run time.
        packages.claude-plugin = pkgs.runCommand "ralph-claude-plugin" { } ''
          mkdir -p "$out"
          cp -r ${self}/.claude-plugin "$out/.claude-plugin"
          cp -r ${self}/skills "$out/skills"
          cp -r ${self}/commands "$out/commands"
          cp -r ${self}/agents "$out/agents"
          cp -r ${self}/hooks "$out/hooks"
          cp -r ${self}/scripts "$out/scripts"
          chmod +x "$out"/scripts/*.sh "$out"/hooks/*.sh

          mkdir -p "$out/bin"
          cp ${config.packages.ralph-loop}/bin/ralph-loop "$out/bin/ralph-loop"
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
