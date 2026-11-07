{ lib, buildGoModule }:
buildGoModule {
  pname = "keeplink-exporter";
  version = "1.0.0";

  src = ../.;
  vendorHash = null;

  buildPhase = ''
    runHook preBuild
    make build
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    mkdir -p $out/bin
    cp bin/exporter-keeplink $out/bin
    runHook postInstall
  '';

  meta = {
    description = "Prometheus exporter for Keeplink KP-9000-9XHML-X";
    homepage = "https://git.cobalt.rocks/cobalt/keeplink-exporter";
    maintainers = [ lib.maintainers.cobalt ];
    mainProgram = "exporter-keeplink";
    platforms = [
      "aarch64-linux"
      "x86_64-linux"
    ];
  };
}
