# maestro-ios-device (BlueShork fork — Maestro 2.10.0)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![macOS](https://img.shields.io/badge/platform-macOS-lightgrey.svg)](https://www.apple.com/macos/)
[![Maestro 2.10.0](https://img.shields.io/badge/Maestro-2.10.0-green.svg)](https://maestro.mobile.dev)

**Run Maestro UI tests on real iOS devices** — iPhone/iPad physical-device testing
via an on-device XCTest driver and automatic USB port forwarding.

> ℹ️ **This is a maintained fork.** The original
> [devicelab-dev/maestro-ios-device](https://github.com/devicelab-dev/maestro-ios-device)
> (Apache-2.0) is frozen and only patches Maestro **2.0.9 – 2.1.0**. This fork adds
> **Maestro 2.10.0** support so it works with current Maestro installs — it's what powers
> physical-iOS support in **[Maestro Deck](#-powering-maestro-deck)**.

> Patch sources: [PR #2856](https://github.com/mobile-dev-inc/Maestro/pull/2856) (real
> iOS device support, closed unmerged upstream) re-applied on the `cli-2.10.0` Maestro tag — see
> [`patches/`](./patches) for the reproducible diff. Originally built by
> [DeviceLab](https://devicelab.dev).

## What it does

1. Patches your existing Maestro install with real-device support (swaps in patched
   2.10.0 jars; your originals are backed up first).
2. Builds and runs the XCTest driver on your iOS device.
3. Sets up USB port forwarding so Maestro can talk to the device.

## Compatibility

| Maestro version | Status |
|-----------------|--------|
| **2.10.0** | ✅ Supported (this fork) |
| 2.5.1 and older | ❌ Dropped — the bundled XCTest runner is the 2.10.0 one |
| Other 2.x | ❌ Not tested |
| 1.x | ❌ Not supported |

> Patches are built against specific Maestro releases. Using an unsupported version
> will fail the `setup` version check.

## Requirements

- macOS (iOS device automation is Apple-toolchain-only — no Windows/Linux)
- [Maestro **2.10.0**](https://maestro.mobile.dev) installed (`maestro --version`)
- Full Xcode (the on-device runner is built with `xcodebuild`)
- An Apple Developer account / Team ID (free works; its cert expires every 7 days)
- An iOS device on iOS 17+ with **Developer Mode** enabled, connected via USB and trusted

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/BlueShork/maestro-ios-device/main/setup.sh | bash
```

This downloads the `maestro-ios-device` binary + the patched 2.10.0 jars + the XCTest
runner, backs up your current Maestro jars, and patches Maestro.

Verify:

```bash
maestro-ios-device --version
```

## Usage

```bash
# 1. Start the device bridge (keep it running):
maestro-ios-device --team-id YOUR_TEAM_ID --device DEVICE_UDID

# 2. In another terminal, run a flow:
maestro --driver-host-port 6001 --device DEVICE_UDID test flow.yaml
```

**Find your Team ID:** `security find-identity -v -p codesigning | grep Developer`
(the 10-char code in parentheses), or Xcode → Settings → Accounts → Team ID.

**Find your device UDID:** `xcrun xctrace list devices`, or Finder → select the iPhone.

## Options

| Flag | Description |
|------|-------------|
| `--team-id` | Apple Developer Team ID (required) |
| `--device` | Device UDID (required) |
| `--driver-host-port` | Local port (default: auto from 6001) |
| `--version` | Show version |
| `--help` | Show help |

## How it works

```
┌─────────────┐     ┌───────────────────┐     ┌─────────────┐
│   Maestro   │────▶│ maestro-ios-device│────▶│ iOS Device  │
│  (patched)  │     │   (port forward)  │     │  (XCTest)   │
└─────────────┘     └───────────────────┘     └─────────────┘
     :6001                                         :22087
```

1. `maestro-ios-device` builds and installs the XCTest runner on the device.
2. The runner serves an HTTP driver on device port `22087`.
3. Port forwarding maps `localhost:6001 → device:22087` (over usbmux — no sudo).
4. Patched Maestro drives the device via `--driver-host-port 6001`.

The `--driver-host-port` flag is `hidden`, so it won't show in `maestro --help` — that's
expected. Stock maestro accepts it too since 2.6, but refuses a port that is already in
use and rebuilds its own driver, so it can't attach to the bridge: the patch is still
required. Patched installs also accept the hidden `--ios-device-patch` flag, which is how
`maestro-ios-device` (and Maestro Deck) tell them apart from stock ones.

## Limitations (real devices)

Some commands are restricted by iOS itself:

| Command | Status | Notes |
|---------|--------|-------|
| `clearState` | ⚠️ No-op | Not permitted on real devices; logs a warning and continues |
| `setLocation` | ⚠️ Limited | Requires extra setup |
| `addMedia` | ❌ Unsupported | iOS restriction |
| `openLink`, `clearKeychain`, `startRecording` | ❌ Unsupported | Not implemented for real devices in Maestro |
| `launchApp` arguments | ⚠️ Ignored | The app launches through XCTest, which takes no launch arguments |

## 🃏 Powering Maestro Deck

This fork is what gives **Maestro Deck** — a desktop app for driving Maestro on
Android, iOS simulators, iOS **physical devices**, and Web — its one-click physical
iPhone support. Maestro Deck auto-installs this bridge, builds the runner, and gives you
a live screen + view-hierarchy inspector + tap/text + flow runs on a real device, right
next to the simulator and Android experiences. 🚀

(The physical-device preview tops out at ~5 fps — Apple removed the OS-level iPhone
screen-capture path on macOS 26 — while simulators stay buttery-smooth via the
CoreSimulator framebuffer.)

## Uninstall / restore

```bash
cp ~/.maestro/backup/* "$(dirname "$(dirname "$(readlink -f "$(which maestro)")")")/lib/"
```

(or just reinstall Maestro 2.10.0 via Homebrew: `brew reinstall maestro`).

## Rebuilding the patched jars

The 2.10.0 `maestro-cli` jar is ~105 MB, over GitHub's file limit, so the repo keeps
only the recompiled classes (`overlay/2.10.0/<jar>/…`). `scripts/build-jars.sh 2.10.0`
downloads the official `maestro.zip`, checks its sha256, swaps those classes into the
stock jars and writes `dist/maestro-jars-2.10.0.zip` (the release workflow runs it).

To regenerate the overlay from source:

```bash
git clone --depth 1 --branch cli-2.10.0 https://github.com/mobile-dev-inc/Maestro.git
cd Maestro && git apply ../patches/maestro-2.10.0-physical-device.patch
JAVA_HOME=<jdk-21> ./gradlew :maestro-cli:installDist
# then copy the classes compiled from the patched files out of
# maestro-cli/build/install/maestro/lib/ into overlay/2.10.0/<jar>/
```

On Apple Silicon without Rosetta, `protoc-gen-grpc-java` 1.50.2 (x86_64 only) fails;
the classes we overlay don't depend on the generated stubs, so the build can skip the
`grpc` protoc plugin and compile against the stock `maestro-client.jar` instead.

## Troubleshooting

- **`Maestro not patched`** — re-run `setup`; confirm `maestro --version` is 2.10.0.
- **`Unsupported Maestro version`** — install Maestro 2.10.0 and re-run `setup`.
- **Build fails / "certificate not trusted"** — usually signing: check the Team ID,
  Developer Mode, and trust the developer cert on the device
  (Settings → General → VPN & Device Management).
- **Stopped working after a Maestro upgrade** — the upgrade overwrote the patched jars;
  re-run `setup` (and make sure you're on 2.10.0).

## License

Apache-2.0 (same as Maestro). This project is not affiliated with, endorsed by, or
connected to mobile.dev, the official Maestro project, or DeviceLab. It patches your
Maestro install to add functionality not yet in the official release. **Use at your own
risk.**
