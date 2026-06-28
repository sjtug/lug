{
  description = "Go development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    flake-utils.url = "github:numtide/flake-utils";
    go2nix = {
      url = "github:numtide/go2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    flake-compat = {
      url = "github:NixOS/flake-compat";
      flake = false;
    };
    pre-commit-hooks = {
      url = "github:cachix/git-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    nix2container = {
      url = "github:nlewo/nix2container";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        inputs.treefmt-nix.flakeModule
        inputs.pre-commit-hooks.flakeModule
      ];

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      perSystem =
        {
          config,
          pkgs,
          lib,
          system,
          ...
        }:
        let
          goVersion =
            let
              goLine = builtins.head (
                builtins.filter (lib.hasPrefix "go ") (lib.splitString "\n" (builtins.readFile ./go.mod))
              );
            in
            lib.removePrefix "go " goLine;

          # go2nix experimental mode (no plugin):
          #   extra-experimental-features = recursive-nix ca-derivations dynamic-derivations
          goEnv = inputs.go2nix.lib.mkGoEnv {
            go = pkgs.go;
            go2nix = inputs.go2nix.packages.${system}.go2nix;
            inherit (pkgs) callPackage;
            nixPackage = pkgs.nix;
          };
        in
        {
          _module.args.pkgs = import inputs.nixpkgs {
            inherit system;
            overlays = [
              (_final: prev: {
                go = prev."go_${lib.versions.major goVersion}_${lib.versions.minor goVersion}";
              })
            ];
          };

          treefmt = {
            projectRootFile = "flake.nix";
            settings.global.excludes = [ ];

            programs = {
              gofumpt.enable = true;
              goimports.enable = true;
              nixfmt.enable = true;
            };
          };

          # https://flake.parts/options/git-hooks-nix.html
          # Example: https://github.com/cachix/git-hooks.nix/blob/master/template/flake.nix
          pre-commit.settings.package = pkgs.prek;
          pre-commit.settings.configPath = ".pre-commit-config.flake.yaml";
          pre-commit.settings.hooks = {
            treefmt.enable = true;
            go2nix = {
              enable = true;
              name = "go2nix";
              description = "Regenerate go2nix.toml lockfile";
              entry =
                let
                  script = pkgs.writeShellScript "go2nix-wrapper" ''
                    exec ${lib.getExe inputs.go2nix.packages.${system}.go2nix} generate .
                  '';
                in
                toString script;
              files = "go\\.(mod|sum)$";
              pass_filenames = false;
            };
          };

          packages.default = config.packages.lug;
          packages.lug = pkgs.callPackage ./package.nix {
            inherit goEnv;
          };
          packages.docker-image = pkgs.callPackage ./docker.nix {
            inherit (config.packages) lug;
            inherit (inputs.nix2container.packages.${system}) nix2container;
          };

          devShells.default = pkgs.mkShell {
            inputsFrom = [
              config.treefmt.build.devShell
              config.pre-commit.devShell
            ];

            shellHook = ''
              echo 1>&2 "Welcome to the development shell!"
            '';

            packages = with pkgs; [
              go
              gopls
              go-tools
              go-mod-upgrade
              govulncheck
              go-junit-report
              go-task
              delve
              inputs.go2nix.packages.${system}.go2nix
            ];
          };
        };
    };
}
