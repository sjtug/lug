{ inputs, ... }:

{
  perSystem =
    {
      config,
      pkgs,
      system,
      ...
    }:
    {
      packages.default = config.packages.lug;
      packages.lug = pkgs.callPackage ../../package.nix { };
      packages.docker-image = pkgs.callPackage ../../docker.nix {
        inherit (config.packages) lug;
        inherit (inputs.nix2container.packages.${system}) nix2container;
      };
    };
}
