{
  description = "Golem Base L3 Store Prototype";
  inputs = {

    nixpkgs = {
      url = "github:NixOS/nixpkgs/nixos-24.11";
    };

    systems.url = "github:nix-systems/default";

  };

  outputs =
    {
      self,
      nixpkgs,
      systems,
      ...
    }@inputs:
    let
      eachSystem =
        f:
        nixpkgs.lib.genAttrs (import systems) (
          system:
          f (
            import nixpkgs {
              inherit system;
              config = {
                allowUnfree = true;
              };
            }
          )
        );

    in
    {
      packages = eachSystem (
        pkgs:
        let
          inherit (pkgs) lib;
        in
        {
          default = pkgs.buildGoModule {
            name = "gb-op-geth";

            src = ./.;

            subPackages = [
              "cmd/abidump"
              "cmd/abigen"
              "cmd/clef"
              "cmd/devp2p"
              "cmd/ethkey"
              "cmd/evm"
              "cmd/geth"
              "cmd/rlpdump"
              "cmd/utils"
            ];

            proxyVendor = true;
            vendorHash = "sha256-hu1ByiMK5vY9+PHsBBAUHt697+Zh9OUM8Gh1UPNXWbA=";

            ldflags = [
              "-s"
              "-w"
            ];

            meta = with lib; {
              description = "";
              homepage = "https://github.com/Golem-Base/op-geth";
              license = licenses.gpl3Only;
              mainProgram = "geth";
            };
          };
        }
      );

      devShells = eachSystem (pkgs: {
        default = pkgs.mkShell {
          shellHook = ''
            # Set here the env vars you want to be available in the shell
          '';
          hardeningDisable = [ "all" ];

          packages = with pkgs; [
            go
            go-tools # staticccheck
            gopls # lsp
            gotools # goimports, ...
            shellcheck
            sqlc
            sqlite
            overmind

          ];
        };
      });
    };
}
