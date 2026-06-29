{ goEnv }:

let
  goAppDrv = goEnv.buildGoApplicationExperimental {
    pname = "lug";
    version = "0.1.0";
    src = ./.;
    goLock = ./go2nix.toml;
    subPackages = [
      "./cli/lug"
      "./cli/lugctl"
    ];

    meta.mainProgram = "lug";
  };
in
goAppDrv.target
