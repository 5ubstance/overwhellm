{ lib
, pkgs
}:

pkgs.buildGoModule {
  pname = "overwhellm";
  version = "0.1.0";

  src = pkgs.lib.cleanSource ../.;

  subPackages = [ "cmd/overwhellm" ];

  vendorHash = null;

  doCheck = false;

  postInstall = ''
    cp ./banner $out/banner
  '';

  meta = with lib; {
    description = "LLM proxy server with token usage tracking";
    homepage = "https://github.com/overwhellm/overwhellm";
    license = licenses.mit;
    mainProgram = "overwhellm";
  };
}