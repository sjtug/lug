{
  buildGoApplication,
}:

buildGoApplication {
  pname = "lug";
  version = "0.1.0";
  pwd = ./.;
  src = ./.;
  modules = ./gomod2nix.toml;

  meta.mainProgram = "lug";
}
