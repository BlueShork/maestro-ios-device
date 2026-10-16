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

## Status

- 2.10.0 jars: the bridge's patch probe and an iOS simulator flow (launchApp, assert, tap)
  pass on the patched jars. Not yet validated on a physical iPhone.
- Physical preview is the driver `/screenshot` poll (~3–5 fps).
