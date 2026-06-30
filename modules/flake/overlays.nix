{ inputs, ... }:

{
  perSystem =
    {
      pkgs,
      lib,
      system,
      ...
    }:
    let
      goVersion =
        let
          goLine = builtins.head (
            builtins.filter (lib.hasPrefix "go ") (lib.splitString "\n" (builtins.readFile ../../go.mod))
          );
        in
        lib.removePrefix "go " goLine;
    in
    {
      _module.args.pkgs = import inputs.nixpkgs {
        inherit system;
        overlays = [
          (_final: prev: {
            go = prev."go_${lib.versions.major goVersion}_${lib.versions.minor goVersion}";

            goEnv = inputs.go2nix.lib.mkGoEnv {
              inherit (pkgs) go callPackage;
              inherit (inputs.go2nix.packages.${system}) go2nix;
              nixPackage = pkgs.nixVersions.nix_2_34;
            };

            inherit (inputs) globset;
          })
        ];
      };
    };
}
