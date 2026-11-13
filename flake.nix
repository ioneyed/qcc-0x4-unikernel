{
  description = "Go + NanoVMs Ops + QEMU environment for the unikernel conference demo";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;
    in {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };

          qemuPackage =
            if pkgs.stdenv.hostPlatform.isLinux
            then pkgs.qemu_kvm
            else pkgs.qemu;

          opsPlatform =
            if pkgs.stdenv.hostPlatform.isDarwin
            then "darwin"
            else "linux";

          opsArch =
            if pkgs.stdenv.hostPlatform.isAarch64
            then "/aarch64"
            else "";

          opsUrl =
            "https://storage.googleapis.com/cli/${opsPlatform}${opsArch}/ops";
        in {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gnumake
              curl
              git
              lsof
              cacert
              qemuPackage
            ];

            shellHook = ''
              export OPS_DIR="''${OPS_DIR:-$HOME/.ops}"
              export PATH="$OPS_DIR/bin:$PATH"

              ops-update() {
                echo "Downloading latest NanoVMs Ops..."

                mkdir -p "$OPS_DIR/bin"

                local tmp
                tmp=$(mktemp)

                if curl --fail --silent --show-error \
                  --location \
                  "${opsUrl}" \
                  --output "$tmp"; then
                  chmod +x "$tmp"
                  mv "$tmp" "$OPS_DIR/bin/ops"
                  echo "Ops installed:"
                  ops version
                else
                  rm -f "$tmp"
                  echo "Failed to download Ops" >&2
                  return 1
                fi
              }

              if [ ! -x "$OPS_DIR/bin/ops" ]; then
                ops-update
              fi

              echo ""
              echo "Unikernel Development Environment"
              echo "--------------------------------"
              echo "Go:   $(go version)"
              echo "QEMU: $(qemu-img --version | head -n 1)"
              echo "Ops:  $(ops version 2>&1 | head -n 1)"
              echo ""
              echo "Run 'ops-update' to download the latest Ops."
            '';
          };
        });
    };
}

