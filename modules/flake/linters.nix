{ inputs, ... }:

{
  imports = [
    inputs.treefmt-nix.flakeModule
    inputs.pre-commit-hooks.flakeModule
  ];

  perSystem =
    {
      config,
      pkgs,
      lib,
      system,
      ...
    }:
    {
      treefmt = {
        projectRootFile = "flake.nix";
        settings.global.excludes = [ ];

        programs = {
          gofumpt.enable = true;
          goimports.enable = true;
          nixfmt.enable = true;
        };
      };

      pre-commit.settings = {
        package = pkgs.prek;
        configPath = ".pre-commit-config.flake.yaml";
        hooks = {
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
      };

      devShells.default = pkgs.mkShell {
        inputsFrom = [
          config.treefmt.build.devShell
          config.pre-commit.devShell
        ];

        packages = with pkgs; [
          go
          gopls
          go-tools
          go-mod-upgrade
          govulncheck
          go-junit-report
          go-task
          delve
          jq
          curl
          inputs.go2nix.packages.${system}.go2nix
        ];
      };
    };
}
