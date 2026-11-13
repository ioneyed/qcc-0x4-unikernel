{
  description = "Go + Ops + QEMU environment for the unikernel conference demo";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      forAllSystems = nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
    in {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
          qemuPackage = if pkgs.stdenv.hostPlatform.isLinux then pkgs.qemu_kvm else pkgs.qemu;
        in {
          default = pkgs.mkShell {
            packages = with pkgs; [ go gnumake ops curl qemuPackage ];
          };
        });
    };
}
