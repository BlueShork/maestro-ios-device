package maestro

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	releaseURL = "https://github.com/BlueShork/maestro-ios-device/releases/latest/download"
	runnerZip  = "maestro-ios-runner.zip"
	runnerDir  = ".maestro/maestro-ios-xctest-runner"
)

// Only 2.10.0: the XCTest runner in assets/ is the 2.10.0 one and is shared by
// every version, so older patched jars would talk to a mismatched driver.
var supportedVersions = []string{"2.10.0"}

// jarsZip is the release asset holding the patched jars for a maestro version.
func jarsZip(version string) string {
	return "maestro-jars-" + version + ".zip"
}

func isSupportedVersion(version string) bool {
	for _, v := range supportedVersions {
		if version == v {
			return true
		}
	}
	return false
}

func runnerBasePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, runnerDir), nil
}

func RunSetup() error {
	fmt.Println("🔧 Maestro iOS Device Setup")
	fmt.Println()

	// Check Maestro installed
	installed, version, err := checkInstalled()
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("Maestro not found. Install from: https://maestro.mobile.dev/")
	}

	fmt.Printf("Detected Maestro: %s\n", version)

	if !isSupportedVersion(version) {
		return fmt.Errorf("Unsupported Maestro version: %s\n\nSupported versions: %s\n\nPlease upgrade or downgrade Maestro to a supported version.", version, strings.Join(supportedVersions, ", "))
	}

	libPath, err := getLibPath()
	if err != nil {
		return fmt.Errorf("failed to find Maestro lib: %w", err)
	}

	runnerPath, err := runnerBasePath()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("📦 Backing up existing JARs...")
	if err := backupJars(libPath); err != nil {
		return fmt.Errorf("failed to backup JARs: %w", err)
	}
	fmt.Println("✅ Backup complete")

	fmt.Println("📥 Downloading JARs...")
	if err := downloadAndExtract(releaseURL+"/"+jarsZip(version), libPath); err != nil {
		return fmt.Errorf("failed to download JARs: %w", err)
	}
	fmt.Println("✅ JARs installed")

	fmt.Println("📥 Downloading iOS runner...")
	// Start from an empty directory: files of an older runner left behind
	// would change its source hash and could shadow the new sources.
	if err := os.RemoveAll(runnerPath); err != nil {
		return err
	}
	if err := os.MkdirAll(runnerPath, 0755); err != nil {
		return err
	}
	if err := downloadAndExtract(releaseURL+"/"+runnerZip, runnerPath); err != nil {
		return fmt.Errorf("failed to download runner: %w", err)
	}
	fmt.Println("✅ iOS runner installed")

	fmt.Println()
	fmt.Println("✅ Setup complete!")
	fmt.Println()
	fmt.Println("Run tests with:")
	fmt.Println("  maestro-ios-device --team-id <ID> --device <UDID>")

	return nil
}

// patchMarkerFlag is a hidden root option that only the patched jars register.
// Stock maestro accepts --driver-host-port since 2.6, so that flag no longer
// tells patched and stock installs apart.
const patchMarkerFlag = "--ios-device-patch"

// The probe exits 0 when the flag parses (help printed) and 2 when picocli
// rejects it, so a missing or broken maestro also reads as not patched.
//
// It boots the maestro JVM (~3 s) on every bridge start, so a positive answer
// is cached against the installed maestro-cli jar (path, size, mtime): any
// maestro upgrade or reinstall rewrites that jar and forces a new probe.
func IsPatched() (bool, error) {
	key := patchCacheKey()
	cache := patchCachePath()
	if key != "" && cache != "" {
		if b, err := os.ReadFile(cache); err == nil && string(b) == key {
			return true, nil
		}
	}
	if err := exec.Command("maestro", patchMarkerFlag, "test", "--help").Run(); err != nil {
		return false, nil
	}
	if key != "" && cache != "" {
		if os.MkdirAll(filepath.Dir(cache), 0755) == nil {
			os.WriteFile(cache, []byte(key), 0644)
		}
	}
	return true, nil
}

func patchCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".maestro", "maestro-ios-device", "patched")
}

// patchCacheKey identifies the maestro-cli jar `maestro` on PATH runs, or ""
// when it can't be found (then the probe always runs).
func patchCacheKey() string {
	libPath, err := getLibPath()
	if err != nil {
		return ""
	}
	jars, _ := filepath.Glob(filepath.Join(libPath, "maestro-cli-*.jar"))
	if len(jars) != 1 {
		return ""
	}
	info, err := os.Stat(jars[0])
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s\n%d\n%d", jars[0], info.Size(), info.ModTime().UnixNano())
}

func GetRunnerPath() (string, error) {
	basePath, err := runnerBasePath()
	if err != nil {
		return "", err
	}

	path := filepath.Join(basePath, "maestro-driver-ios.xcodeproj")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("iOS runner not found. Run: maestro-ios-device setup")
	}

	return basePath, nil
}

func checkInstalled() (bool, string, error) {
	out, err := exec.Command("maestro", "--version").Output()
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "executable file not found") {
			return false, "", nil
		}
		return false, "", err
	}
	return true, parseVersion(string(out)), nil
}

func parseVersion(out string) string {
	out = strings.TrimSpace(out)

	for _, prefix := range []string{"cli version: ", "version: ", "CLI "} {
		if idx := strings.Index(strings.ToLower(out), strings.ToLower(prefix)); idx != -1 {
			rest := out[idx+len(prefix):]
			if fields := strings.Fields(rest); len(fields) > 0 {
				return fields[0]
			}
		}
	}

	if m := regexp.MustCompile(`\d+\.\d+\.\d+`).FindString(out); m != "" {
		return m
	}

	return out
}

func getLibPath() (string, error) {
	scriptPath, err := exec.Command("which", "maestro").Output()
	if err != nil {
		return "", fmt.Errorf("maestro not in PATH")
	}

	script := strings.TrimSpace(string(scriptPath))
	if resolved, err := filepath.EvalSymlinks(script); err == nil {
		script = resolved
	}

	if content, err := os.ReadFile(script); err == nil {
		if libPath := findLibInScript(string(content), filepath.Dir(script)); libPath != "" {
			return libPath, nil
		}
	}

	// Fallback 1: lib is sibling to bin
	binDir := filepath.Dir(script)
	libPath := filepath.Join(filepath.Dir(binDir), "lib")
	if _, err := os.Stat(libPath); err == nil {
		return libPath, nil
	}

	// Fallback 2: ~/.maestro/lib
	if home, err := os.UserHomeDir(); err == nil {
		libPath = filepath.Join(home, ".maestro", "lib")
		if _, err := os.Stat(libPath); err == nil {
			return libPath, nil
		}
	}

	return "", fmt.Errorf("lib directory not found")
}

func findLibInScript(content, scriptDir string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "CLASSPATH") && strings.Contains(line, "/lib/") {
			if idx := strings.Index(line, "/lib/"); idx > 0 {
				start := strings.LastIndexAny(line[:idx], "=\"'") + 1
				pathPart := line[start : idx+4]

				if strings.HasPrefix(pathPart, "$") {
					return filepath.Join(filepath.Dir(scriptDir), "lib")
				}
				if filepath.IsAbs(pathPart) {
					return pathPart
				}
			}
		}
	}
	return ""
}

func backupJars(libPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	backupDir := filepath.Join(home, ".maestro", "backup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(libPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "maestro") || !strings.HasSuffix(name, ".jar") {
			continue
		}
		src := filepath.Join(libPath, name)
		dst := filepath.Join(backupDir, name)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to backup %s: %w", name, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func downloadAndExtract(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "maestro-*.zip")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	return unzip(tmpName, dest)
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Security: prevent zip slip
		target := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		if err := extractFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

func extractFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
