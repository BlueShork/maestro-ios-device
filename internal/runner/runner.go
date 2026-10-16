package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/maestro-ios-device/internal/maestro"
)

const (
	DevicePort     = uint16(22087)
	buildTimeout   = 10 * time.Minute
	startupTimeout = 90 * time.Second
	// readyPoll is how often Start probes the driver's /status while the
	// runner comes up. Each probe is a local TCP connect through the forward.
	readyPoll = 200 * time.Millisecond
	// cacheRoot holds one build per device + team (relative to $HOME), plus
	// the Swift packages shared by all of them.
	cacheRoot = ".maestro/maestro-ios-device"
	stampFile = "runner.sha256"
)

type Runner struct {
	deviceUDID string
	teamID     string
	buildDir   string
	cmd        *exec.Cmd
	exited     chan struct{}
	logFile    *os.File
}

func New(deviceUDID, teamID string) *Runner {
	return &Runner{
		deviceUDID: deviceUDID,
		teamID:     teamID,
	}
}

// Build compiles the XCTest runner for the device, or reuses the previous
// build when neither the runner source nor the team changed. The build lives
// in a fixed per-device directory so later connections skip xcodebuild
// entirely (a cold build is ~30 s, a fresh derivedData pays it every time).
func (r *Runner) Build(ctx context.Context) error {
	runnerPath, err := maestro.GetRunnerPath()
	if err != nil {
		return err
	}

	root, err := cacheDir()
	if err != nil {
		return err
	}
	r.buildDir = filepath.Join(root, "build", r.deviceUDID+"-"+r.teamID)
	if err := os.MkdirAll(filepath.Join(r.buildDir, "logs"), 0755); err != nil {
		return err
	}

	hash, err := sourceHash(runnerPath)
	if err != nil {
		return fmt.Errorf("hash runner source: %w", err)
	}
	stamp := filepath.Join(r.buildDir, stampFile)
	if prev, err := os.ReadFile(stamp); err == nil && string(prev) == hash {
		if _, err := r.findXctestrun(); err == nil {
			fmt.Println("♻️  Runner already built for this device")
			return nil
		}
	}
	// Drop the stamp first: an interrupted build must not look up to date.
	os.Remove(stamp)

	logPath := filepath.Join(r.buildDir, "logs", "build.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()

	buildCtx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()

	cmd := exec.CommandContext(buildCtx, "xcodebuild",
		"build-for-testing",
		"-project", filepath.Join(runnerPath, "maestro-driver-ios.xcodeproj"),
		"-scheme", "maestro-driver-ios",
		"-destination", r.destination(),
		"-derivedDataPath", r.buildOut(),
		// Shared across devices so FlyingFox is fetched once per machine.
		"-clonedSourcePackagesDirPath", filepath.Join(root, "packages"),
		// Lets Xcode register a new iPhone and create its development profile
		// instead of failing with "No profiles for … were found".
		"-allowProvisioningUpdates",
		"-skipPackagePluginValidation",
		"-skipMacroValidation",
		fmt.Sprintf("DEVELOPMENT_TEAM=%s", r.teamID),
		"COMPILER_INDEX_STORE_ENABLE=NO",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	fmt.Println("🔨 Building the runner for this device (first time only, ~30 s)...")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed:\n%s\n\nFull log: %s", tailLog(logPath, 20), logPath)
	}

	if _, err := r.findXctestrun(); err != nil {
		return err
	}
	if err := os.WriteFile(stamp, []byte(hash), 0644); err != nil {
		return err
	}

	fmt.Println("✅ Build complete")
	return nil
}

func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, cacheRoot), nil
}

// sourceHash fingerprints the runner source tree (paths + contents), skipping
// Xcode's per-user and build output, so a `setup` that ships a new runner
// invalidates every cached build.
func sourceHash(root string) (string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && (name == "xcuserdata" || name == "Build" || name == ".build" || name == ".swiftpm") {
			return filepath.SkipDir
		}
		if !d.IsDir() && name != ".DS_Store" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	h := sha256.New()
	for _, path := range files {
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(h, "%s\x00", rel)
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (r *Runner) buildOut() string {
	return filepath.Join(r.buildDir, "build")
}

func (r *Runner) destination() string {
	return fmt.Sprintf("id=%s", r.deviceUDID)
}

func (r *Runner) findXctestrun() (string, error) {
	pattern := filepath.Join(r.buildOut(), "Build", "Products", "*.xctestrun")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return "", fmt.Errorf("no xctestrun file found")
	}
	return matches[0], nil
}

// Start launches the runner and returns once its driver answers /status on
// localPort (the port forward must already be up), or fails fast on the
// errors xcodebuild reports in its log.
func (r *Runner) Start(ctx context.Context, localPort uint16) error {
	xctestrun, err := r.findXctestrun()
	if err != nil {
		return err
	}

	logPath := filepath.Join(r.buildDir, "logs", "runner.log")
	r.logFile, err = os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	r.cmd = exec.CommandContext(ctx, "xcodebuild",
		"test-without-building",
		"-xctestrun", xctestrun,
		"-destination", r.destination(),
		"-derivedDataPath", r.buildOut(),
	)
	r.cmd.Stdout = r.logFile
	r.cmd.Stderr = r.logFile

	fmt.Println("▶️  Starting runner...")

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start runner: %w", err)
	}
	r.exited = make(chan struct{})
	go func() {
		r.cmd.Wait()
		close(r.exited)
	}()

	if err := r.waitForStartup(logPath, localPort); err != nil {
		r.Stop()
		return err
	}

	fmt.Println("✅ Runner started")
	return nil
}

func (r *Runner) Stop() {
	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Kill()
	}
	if r.logFile != nil {
		r.logFile.Close()
	}
}

// Cleanup stops the runner. The build directory is kept: it is the cache
// that makes the next connection skip xcodebuild.
func (r *Runner) Cleanup() {
	r.Stop()
}

// DriverReady reports whether the XCTest driver answers /status through the
// local forward. Used both to wait for a fresh runner and to reuse one that is
// still running on the device from a previous session.
func DriverReady(localPort uint16) bool {
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/status", localPort))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (r *Runner) waitForStartup(logPath string, localPort uint16) error {
	timeout := time.After(startupTimeout)
	ticker := time.NewTicker(readyPoll)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if DriverReady(localPort) {
				return nil
			}
			content, err := os.ReadFile(logPath)
			if err != nil {
				continue
			}
			if err := checkLog(string(content), logPath); err != nil {
				return err
			}
		case <-r.exited:
			return fmt.Errorf("runner exited:\n%s\n\nFull log: %s", tailLog(logPath, 20), logPath)
		case <-timeout:
			return fmt.Errorf("startup timeout (90s):\n%s\n\nFull log: %s", tailLog(logPath, 20), logPath)
		}
	}
}

// checkLog turns the xcodebuild failures we know into readable errors. Success
// is no longer read from the log: the driver answering /status is the signal.
func checkLog(log, logPath string) error {
	if strings.Contains(log, "Developer App Certificate is not trusted") {
		return fmt.Errorf("certificate not trusted - trust it in Settings > General > VPN & Device Management")
	}
	if strings.Contains(log, "Testing failed:") {
		return fmt.Errorf("runner failed:\n%s\n\nFull log: %s", tailLog(logPath, 20), logPath)
	}
	return nil
}

func tailLog(path string, lines int) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(could not read log: %s)", err)
	}
	allLines := strings.Split(string(content), "\n")
	if len(allLines) <= lines {
		return string(content)
	}
	return strings.Join(allLines[len(allLines)-lines:], "\n")
}
