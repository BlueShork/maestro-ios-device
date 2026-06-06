# Fork notes — maestro-ios-device (self-maintained, maestro 2.5.1)

Fork of [devicelab-dev/maestro-ios-device](https://github.com/devicelab-dev/maestro-ios-device)
(Apache-2.0), which is frozen/deprecated and only patches maestro 2.0.9–2.1.0. This fork
adds **maestro 2.5.1** support so maestro-deck can drive a physical iPhone on current maestro.

## What changed vs upstream

- `internal/maestro/maestro.go`
  - `supportedVersions` += `"2.5.1"`.
  - `IsPatched()` relaxed: maestro 2.5.1's `--driver-host-port` flag is registered with
    `hidden = true`, so it does NOT appear in `maestro --help`. We now probe
    `maestro test --help` and accept the flag parsing (no "Unknown option" /
    "Unmatched argument") as "patched".
- `assets/jars/2.5.1/` — patched maestro 2.5.1 jars (full runtime set; 4 are patched:
  `maestro-cli-2.5.1`, `maestro-client`, `maestro-ios-driver`, `maestro-ios`).
- `patches/maestro-2.5.1-physical-device.patch` — the source diff applied to the maestro
  `cli-2.5.1` tag to produce those jars (PR #2856 + a back-ported `--driver-host-port`
  CLI `@Option`). Reproduce with:
  ```
  git clone --depth 1 --branch cli-2.5.1 https://github.com/mobile-dev-inc/Maestro.git
  cd Maestro && git apply ../patches/maestro-2.5.1-physical-device.patch
  JAVA_HOME=<jdk21> ./gradlew :maestro-cli:installDist
  # jars land in maestro-cli/build/install/maestro/lib/
  ```

## Status

Validated on real hardware (iPhone 17 Pro, iOS 26.5.1): device lists in maestro-deck,
screen preview + view-hierarchy inspector + tap/text work. Physical preview is the
driver `/screenshot` poll (~3fps); smooth 60fps is simulator-only (CoreSimulator
framebuffer) and a possible future enhancement via AVFoundation/CoreMediaIO.

## TODO (Phase 2)

- Push to a new repo (private first, public later); update `releaseURL` in
  `internal/maestro/maestro.go` and `release.yml` to zip `assets/jars/2.5.1`.
- Re-point maestro-deck auto-install (`src-tauri/src/ipc/commands.rs`) at this repo
  (note: the auto-install download is unauthenticated → only works once the repo is public).
