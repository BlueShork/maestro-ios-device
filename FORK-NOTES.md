# Fork notes — maestro-ios-device (self-maintained, maestro 2.10.0)

Fork of [devicelab-dev/maestro-ios-device](https://github.com/devicelab-dev/maestro-ios-device)
(Apache-2.0), which is frozen/deprecated and only patches maestro 2.0.9–2.1.0. This fork
adds **maestro 2.10.0** support so maestro-deck can drive a physical iPhone on current
maestro. (It patched 2.5.1 before; 2.5.1 support was dropped with the 2.10.0 runner.)

Upstream PR #2856 (real iOS device support) was closed unmerged, and stock 2.10.0 still
cannot run a flow on a physical iPhone: `launchApp` and `clearState` hit
`TODO("Not yet implemented")` in `DeviceControlIOSDevice`.

## What changed vs upstream

- `internal/maestro/maestro.go`
  - `supportedVersions` = `["2.10.0"]` only: the XCTest runner in `assets/` is the 2.10.0
    one and is shared by every version.
  - jars are downloaded as `maestro-jars-<version>.zip`, so an older bridge binary can
    never fetch jars for a version it doesn't know.
  - `IsPatched()` probes the hidden `--ios-device-patch` flag (exit 0 = patched). Stock
    maestro accepts `--driver-host-port` since 2.6, so that flag no longer works as a probe.
- `patches/maestro-2.10.0-physical-device.patch` — source diff against the maestro
  `cli-2.10.0` tag:
  - `TestCommand.selectPort`: use a user `--driver-host-port` verbatim (stock 2.10.0 throws
    when the port is busy, and the bridge's forward is exactly what holds it).
  - `MaestroSessionManager`: with `--driver-host-port`, skip the session-store check, the
    driver build/validation and the `isShutdown()` probe.
  - `DeviceService`: list iPhones with Developer Mode enabled, not only tunnel-connected ones.
  - `LocalIOSDevice.launch`: fall back to XCTest when devicectl's `launch` is unimplemented.
  - `DeviceControlIOSDevice`: `clearAppState` logs a warning instead of crashing; `close`
    keeps the runner app installed so sessions can reuse it.
  - `App`: hidden `--ios-device-patch` marker flag.
  (The `--driver-host-port` option itself is upstream since 2.6.)
- `overlay/2.10.0/<jar>/` — the classes compiled from those files. The 2.10.0
  `maestro-cli` jar is ~105 MB (over GitHub's limit), so `scripts/build-jars.sh 2.10.0`
  builds the release zip from the official `maestro.zip` (sha256-pinned) + this overlay.
- `assets/maestro-ios-xctest-runner/` — the 2.10.0 runner source (team ID blanked).
- `assets/jars/<old versions>/` — kept for history, no longer shipped.

## Connection speed

Measured on an iPhone 17 Pro (iOS 26.6.1), bridge start → Ready:

| Case | Before | Now |
|------|--------|-----|
| First connection of a device (no cache) | ~60 s | ~22 s |
| Reconnect, runner build cached | ~60 s | ~4 s |
| Reconnect, runner still running (bridge killed, xcodebuild survived) | ~60 s | ~0.2 s |

- The runner build lives in `~/.maestro/maestro-ios-device/build/<udid>-<team>/` with a
  sha256 stamp of the runner source; Swift packages are shared in `…/packages`.
  `-allowProvisioningUpdates` lets Xcode create the profile for a new iPhone.
- The forward starts first; a runner that already answers `/status` is reused as is.
- Readiness = the driver answering `/status` (polled every 200 ms), not log scraping.
- The maestro patch probe (a ~3 s JVM boot) runs in parallel and its positive result is
  cached against the installed `maestro-cli` jar (path, size, mtime).
- Runner `IsScreenStaticHandler` ignores the top 64 pt (status bar / Dynamic Island) and
  hashes raw pixels: on a real iPhone the clock and live activities kept the whole-screen
  hash changing, so every Maestro settle wait ran to its 3 s timeout. A 6-step flow went
  from 31 s to 17 s.

## Status

- 2.10.0 validated on an iPhone 17 Pro (iOS 26.6.1): launchApp (+ clearState), assert,
  tapOn, back button; also an iOS simulator flow on the patched jars.
- Physical preview is the driver `/screenshot` poll (~3–5 fps).
