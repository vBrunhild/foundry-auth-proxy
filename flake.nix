{
  description = "foundry-auth-proxy";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = {nixpkgs, ...}: let
    systems = ["x86_64-linux" "aarch64-linux" "aarch64-darwin"];
    forAllSystems = f:
      nixpkgs.lib.genAttrs systems
      (system: f nixpkgs.legacyPackages.${system});

    version = "0.1.0";
  in {
    packages = forAllSystems (pkgs: let
      foundry-auth-proxy = pkgs.buildGoModule {
        inherit version;
        pname = "foundry-auth-proxy";

        src = ./.;
        vendorHash = null;
        subPackages = ["cmd/foundry-auth-proxy"];

        env.CGO_ENABLED = "0";
        ldflags = ["-s" "-w"];
        flags = ["-trimpath"];

        meta.mainProgram = "foundry-auth-proxy";
      };

      image = pkgs.dockerTools.buildImage {
        name = "foundry-auth-proxy";
        tag = version;

        config = {
          Entrypoint = ["${foundry-auth-proxy}/bin/foundry-auth-proxy"];
          Cmd = ["-config" "/etc/foundry-auth-proxy/config.json"];
          ExposedPorts."8080/tcp" = {};
          User = "1000:1000";
        };
      };
    in
      {
        default = foundry-auth-proxy;
      }
      // pkgs.lib.optionalAttrs pkgs.stdenv.isLinux {inherit image;});

    devShells = forAllSystems (pkgs: {
      default = pkgs.mkShell {
        packages = with pkgs; [
          go
          gopls
          gotools
          golangci-lint
          delve
          jujutsu
        ];
      };
    });

    formatter = forAllSystems (pkgs: pkgs.alejandra);
  };
}
