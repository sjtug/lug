_: {
  imports = [
    ./overlays.nix
    ./linters.nix
    ./packages.nix
  ];

  systems = [
    "x86_64-linux"
    "aarch64-linux"
    "aarch64-darwin"
  ];
}
