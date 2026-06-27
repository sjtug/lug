{
  description = "Go development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
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
        in
        {
          _module.args.pkgs = import inputs.nixpkgs {
            inherit system;
            overlays = [
              inputs.gomod2nix.overlays.default
              (_final: prev: {
                go = prev."go_${lib.versions.major goVersion}_${lib.versions.minor goVersion}";
              })
            ];
          };

          treefmt = {
            projectRootFile = "flake.nix";
            settings.global.excludes = [ ];

            programs = {
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

            golangci-lint = {
              enable = true;
              name = "golangci-lint";
              description = "Run golangci-lint on Go files";
              entry =
                let
                  script = pkgs.writeShellScript "golangci-lint-wrapper" ''
                    # Skip in Nix build sandbox (nix flake check) — no network access to download Go modules
                    if [ -n "''${NIX_BUILD_TOP:-}" ]; then
                      echo "golangci-lint: skipping (Nix sandbox, no network)"
                      exit 0
                    fi
                    export PATH="${pkgs.go}/bin:$PATH"
                    # golangci-lint can only process one Go package per invocation;
                    # group files by directory and run per-directory.
                    dirs=$(printf "%s\n" "$@" | xargs -n1 dirname | sort -u)
                    for dir in $dirs; do
                      ${pkgs.golangci-lint}/bin/golangci-lint run --fix "./$dir"
                    done
                  '';
                in
                toString script;
              files = "\\.go$";
              excludes = [ "vendor/" ];
              pass_filenames = true;
            };

            gomod2nix = {
              enable = true;
              name = "gomod2nix";
              description = "Regenerate gomod2nix.toml";
              entry =
                let
                  script = pkgs.writeShellScript "gomod2nix-wrapper" ''
                    # Skip in Nix build sandbox (nix flake check) — no network access
                    if [ -n "''${NIX_BUILD_TOP:-}" ]; then
                      echo "gomod2nix: skipping (Nix sandbox, no network)"
                      exit 0
                    fi
                    exec ${lib.getExe' pkgs.gomod2nix "gomod2nix"} generate
                  '';
                in
                toString script;
              files = "(go\\.(mod|sum)|gomod2nix\\.toml)$";
              pass_filenames = false;
            };
          };

          packages.default = pkgs.callPackage ./package.nix {
            inherit (inputs.gomod2nix.legacyPackages.${system}) buildGoApplication;
          };
          packages.docker-image = pkgs.callPackage ./docker.nix {
            lug = config.packages.default;
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
              (mkGoEnv { pwd = ./.; })
              gopls
              gomod2nix
              gotools
              go-mod-upgrade
              govulncheck
              go-junit-report
              go-task
              delve
            ];
          };
        };
    };
}
