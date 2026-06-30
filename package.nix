{
  lib,
  globset,
  goEnv,
  symlinkJoin,
}:

let
  version = "0.1.0";
  goAppDrv = goEnv.buildGoApplicationExperimental {
    pname = "lug";
    inherit version;
    src = lib.fileset.toSource {
      root = ./.;
      fileset = globset.lib.globs ./. [
        "**/*.go"
        "**/*.tar"
        "go.mod"
        "go.sum"
      ];
    };
    goLock = ./go2nix.toml;
    subPackages = [
      "./cli/lug"
      "./cli/lugctl"
    ];

    meta.mainProgram = "lug";
  };
in
symlinkJoin {
  name = "lug-${version}";
  paths = [ goAppDrv.target ];
}
