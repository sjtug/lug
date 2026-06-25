{
  lib,
  nix2container,
  dockerTools,
  lug,
  bash,
  rsync,
  coreutils,
  findutils,
  gnugrep,
  gawk,
  gnused,
  curl,
  jq,
  jsonApiPort ? 7001,
  exporterPort ? 8081,
  configPath ? "/config.yaml",
}:
let
  # Runtime tools exposed on PATH inside the container (with their /bin).
  runtimeDeps = [
    bash
    rsync
    coreutils
    findutils
    gnugrep
    gawk
    gnused
    curl
    jq
  ];
  # Nest each layer so prior layers become dependencies of later ones,
  # avoiding redundant dependencies in the final image. Mirrors the
  # frontend docker.nix layout.
  foldImageLayers =
    let
      mergeToLayer =
        priorLayers: component:
        assert builtins.isList priorLayers;
        assert builtins.isAttrs component;
        let
          layer = nix2container.buildLayer (
            component
            // {
              layers = priorLayers;
            }
          );
        in
        priorLayers ++ [ layer ];
    in
    layers: lib.foldl mergeToLayer [ ] layers;
in
nix2container.buildImage {
  name = "lug";
  tag = "latest";

  copyToRoot = [
    dockerTools.caCertificates
  ];

  layers =
    let
      layerDefs = [
        # Runtime tools the shell_script workers exec (rsync.sh, test.sh, ...).
        # Kept in one layer so the (much smaller) lug binary layer can be
        # re-used / invalidated independently.
        { deps = runtimeDeps; }
        { deps = [ lug ]; }
      ];
    in
    foldImageLayers layerDefs;

  config = {
    # Launch the binary directly (no shell) so lug is PID 1 — signal
    # forwarding for graceful shutdown (Phase 1) lands on the daemon itself,
    # and `podman run lug:latest <args>` appends args to the binary. With
    # `lug` on PATH below it is also discoverable from a shell / `podman exec`.
    Entrypoint = [ "${lib.getExe lug}" ];
    Cmd = [
      "-c"
      configPath
    ];
    Env = [
      # nix2container sets no PATH by default. Expose the lug binary and the
      # runtime tools the shell_script workers exec on PATH (rsync.sh, ...).
      "PATH=${lib.makeBinPath ([ lug ] ++ runtimeDeps)}"
    ];
    ExposedPorts = {
      "${toString jsonApiPort}/tcp" = { };
      "${toString exporterPort}/tcp" = { };
    };
  };
}
