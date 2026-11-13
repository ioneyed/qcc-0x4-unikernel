# Unikernel blast-radius demo

A talk-ready demonstration of running the same deliberately vulnerable Go web server directly on a host and as a [Nanos](https://nanos.org/) unikernel. [Ops](https://ops.city/) packages the static binary and launches it through QEMU.

> **Warning:** this app intentionally exposes arbitrary file-read and command-execution endpoints. Run it only on a local demo machine, never on a public interface. The endpoints are disabled unless `DEMO_UNSAFE=1`; the demo Make targets opt in deliberately.

## The 60-second path

After installing the prerequisites below:

```sh
make check
make test
make run-native
```

In another terminal:

```sh
curl --get --data-urlencode 'path=/etc/passwd' http://localhost:8080/read
curl --get --data-urlencode 'cmd=id' http://localhost:8080/exec
```

Stop the native process with `Ctrl-C`, then launch the application as a unikernel:

```sh
make run-unikernel
```

Repeat the two `curl` commands. The vulnerable handlers are still reachable, but the host account database is replaced by a minimal guest file (typically one `root` entry ending in `/bin/nobash`), and the command attempt fails because Nanos does not implement process creation. Open <http://localhost:8080> for the built-in demo page.

The first Ops launch downloads a Nanos release and can take a few minutes. Do it once before going on stage.

On Linux, both runs use the exact same `build/vulnapp` ELF. On macOS, the native comparison uses a host-native Go build while Ops uses the Linux ELF; the source and configuration remain the same.

## Install prerequisites

You need Go 1.22 or newer, Make, QEMU 7 or newer, and Ops. Ops uses QEMU's user-mode networking for this local demo and forwards host port 8080 to the unikernel.

### NixOS / Nix

The included flake supplies everything:

```sh
nix develop
make check
```

Or install the tools into your profile:

```sh
nix profile install nixpkgs#go nixpkgs#gnumake nixpkgs#qemu nixpkgs#ops
```

### macOS (Homebrew)

```sh
brew install go qemu
brew tap nanovms/homebrew-ops
brew install nanovms/ops/ops
```

Both Intel and Apple Silicon work. The Makefile builds for the host architecture so Ops can use hardware acceleration.

### Debian / Ubuntu

```sh
sudo apt-get update
sudo apt-get install -y build-essential curl golang-go qemu-kvm qemu-utils
curl https://ops.city/get.sh -sSfL | sh
```

If your distribution's `golang-go` is older than 1.22, install a current release from <https://go.dev/doc/install>. If `ops` is not found after installation, start a new shell or add the installer-reported directory to `PATH`.

### Fedora

```sh
sudo dnf install -y @development-tools curl golang qemu-kvm qemu-img
curl https://ops.city/get.sh -sSfL | sh
```

The pipe-to-shell command is Ops's official quick installer. If you prefer to inspect artifacts first, use the [Ops downloads page](https://ops.city/downloads) or the Homebrew formula.

### Verify hardware acceleration

On Linux, check KVM access:

```sh
test -r /dev/kvm -a -w /dev/kvm && echo "KVM available" || echo "KVM unavailable; QEMU may be slower"
```

On macOS, QEMU uses Apple's Hypervisor Framework. Ops prints a warning if acceleration is unavailable; software emulation is slower but adequate for this demo.

## Commands

| Command | Purpose |
| --- | --- |
| `make check` | Verify Go, the architecture-appropriate QEMU executable, and Ops |
| `make test` | Test the app and confirm unsafe endpoints are off by default |
| `make build` | Produce a stripped, static Linux ELF at `build/vulnapp` |
| `make run-native` | Run the host comparison with vulnerabilities enabled |
| `make image` | Create a bootable Nanos image without launching it |
| `make run-unikernel` | Build and boot the image, forwarding port 8080 |

Override the demo port with `make run-native PORT=9090` and then `make run-unikernel PORT=9090`. The Makefile renders the value into `ops.json` so the host forwarding and guest listener stay aligned.

## What the demo proves—and what it does not

The native process inherits a general-purpose operating system: host files, users, utilities, and a shell. Nanos packages one application with its selected files and a small set of kernel-provided compatibility files. There is no SSH daemon, package manager, or general-purpose shell in this image, so the same application bugs have less ambient machinery to abuse.

This is **blast-radius reduction**, not a vulnerability fix:

- The arbitrary file-read can still read every file deliberately packaged with the app.
- The command endpoint is still unauthenticated attacker-controlled input.
- Application-level data theft, network abuse, denial of service, and logic bugs remain possible.
- The hypervisor boundary and smaller image reduce attack surface; they do not make unsafe code safe.

That distinction is the key line for the talk: **a unikernel complements secure application design; it does not replace it.**

## How it works

`make build` sets `CGO_ENABLED=0` and `GOOS=linux`, producing the standalone ELF format Ops expects. `ops.json` enables the intentionally vulnerable routes inside the guest and allocates one CPU with 128 MiB RAM. `ops run -p 8080` builds a Nanos disk image, starts QEMU, and configures local port forwarding. Ops config rendering keeps the guest port aligned when `PORT` is overridden.

The application limits file and command output to 64 KiB and command runtime to two seconds so a typo is less likely to derail a live demo. It is still intentionally unsafe.

## Troubleshooting

**The first run appears stuck:** Ops may be downloading Nanos. Wait for the download and boot messages; pre-warm it before the talk.

**Port 8080 is already in use:** stop the native app before starting the unikernel. Find listeners with `lsof -i :8080` (macOS) or `ss -ltnp 'sport = :8080'` (Linux).

**QEMU says acceleration is unavailable:** verify `/dev/kvm` permissions on Linux or that virtualization is enabled in firmware. The demo can still run under software emulation, just more slowly.

**Apple Silicon image has the wrong architecture:** run `go env GOARCH`; it should print `arm64`. You can force it with `make run-unikernel ARCH=arm64`.

**Ops cannot find a release:** confirm internet access, run `ops version`, and retry once before the talk. The downloaded release is cached under `~/.ops`.

## References

- [Ops installation and usage](https://github.com/nanovms/ops)
- [Ops environment setup and QEMU requirement](https://docs.ops.city/ops/developer/env-setup)
- [Ops Go static-server example](https://docs.ops.city/ops/examples)
- [Ops networking](https://docs.ops.city/ops/networking)
- [Nanos source and architecture](https://github.com/nanovms/nanos)
