package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

const (
	checkInterval   = 12 * time.Hour
	githubOwner     = "sergiocarracedo"
	githubRepo      = "skill-organizer"
	githubLatestURL = "https://api.github.com/repos/sergiocarracedo/skill-organizer/releases/latest"
	defaultPageURL  = "https://github.com/sergiocarracedo/skill-organizer/releases/latest"
	userAgent       = "skill-organizer-cli"
)

type InstallMethod string

const (
	InstallMethodUnknown  InstallMethod = "unknown"
	InstallMethodNPM      InstallMethod = "npm"
	InstallMethodHomebrew InstallMethod = "homebrew"
	InstallMethodDirect   InstallMethod = "direct"
)

type releaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type latestRelease struct {
	version  string
	pageURL  string
	assetURL string
}

type VersionInfo struct {
	CurrentVersion  string
	LatestVersion   string
	LatestPageURL   string
	UpdateCommand   string
	InstallMethod   InstallMethod
	UpdateAvailable bool
}

func MaybeNotify(ctx context.Context, currentVersion string, stdout io.Writer) {
	if shouldSkipPeriodicCheck() {
		return
	}

	info, err := Check(ctx, currentVersion)
	if err != nil || !info.UpdateAvailable {
		return
	}

	_, _ = fmt.Fprintf(stdout, "\nNew skill-organizer version available: %s -> %s\n", info.CurrentVersion, info.LatestVersion)
	_, _ = fmt.Fprintf(stdout, "Run `skill-organizer self-update` to update.\n")
	if info.InstallMethod == InstallMethodDirect {
		_, _ = fmt.Fprintf(stdout, "Direct installs must be updated manually from the release page: %s\n", info.LatestPageURL)
	}
	_, _ = fmt.Fprintln(stdout)
}

func Check(ctx context.Context, currentVersion string) (VersionInfo, error) {
	method := DetectInstallMethod()
	cachePath, err := configpkg.CachePath()
	if err != nil {
		return VersionInfo{}, err
	}

	cache, err := configpkg.LoadUpdateCacheOrDefault(cachePath)
	if err != nil {
		return VersionInfo{}, err
	}

	now := time.Now().UTC()
	if cached, ok := cachedVersionInfo(cache, currentVersion, method, now); ok {
		return cached, nil
	}

	latest, err := fetchLatestRelease(ctx)
	if err != nil {
		return VersionInfo{}, err
	}

	info := VersionInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   latest.version,
		LatestPageURL:   latest.pageURL,
		UpdateCommand:   updateCommandFor(method),
		InstallMethod:   method,
		UpdateAvailable: isRemoteNewer(currentVersion, latest.version),
	}

	cache = configpkg.UpdateCache{
		LastCheckedAt: now.Format(time.RFC3339),
		LatestVersion: latest.version,
		LatestPageURL: latest.pageURL,
	}
	if err := configpkg.SaveUpdateCache(cachePath, cache); err != nil {
		return info, nil
	}

	return info, nil
}

func Run(ctx context.Context, currentVersion string, stdout, stderr io.Writer) error {
	method := DetectInstallMethod()
	if method == InstallMethodUnknown {
		method = InstallMethodDirect
	}

	latest, err := fetchLatestRelease(ctx)
	if err != nil {
		return err
	}

	if !isRemoteNewer(currentVersion, latest.version) {
		_, _ = fmt.Fprintf(stdout, "skill-organizer is up to date (%s).\n", currentVersion)
		return nil
	}

	_, _ = fmt.Fprintf(stdout, "Updating skill-organizer from %s to %s using %s...\n", currentVersion, latest.version, method)

	switch method {
	case InstallMethodNPM:
		if err := runCommand(ctx, stdout, stderr, commandForCurrentPlatform("npm", "npm.cmd"), "i", "-g", "skill-organizer@latest"); err != nil {
			return err
		}
	case InstallMethodHomebrew:
		if err := runCommand(ctx, stdout, stderr, "brew", "upgrade", "skill-organizer"); err != nil {
			return err
		}
	case InstallMethodDirect:
		if err := updateDirectBinary(ctx, latest); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported install method: %s", method)
	}

	if err := clearCachedUpdateState(); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(stdout, "Update completed.")
	return nil
}

func DetectInstallMethod() InstallMethod {
	if strings.TrimSpace(os.Getenv("npm_execpath")) != "" || strings.TrimSpace(os.Getenv("npm_config_user_agent")) != "" {
		return InstallMethodNPM
	}
	if strings.TrimSpace(os.Getenv("HOMEBREW_PREFIX")) != "" || strings.TrimSpace(os.Getenv("HOMEBREW_CELLAR")) != "" {
		return InstallMethodHomebrew
	}

	exePath, err := os.Executable()
	if err != nil {
		return InstallMethodUnknown
	}

	normalized := filepath.ToSlash(strings.ToLower(exePath))
	if strings.Contains(normalized, "/node_modules/") {
		return InstallMethodNPM
	}
	if strings.Contains(normalized, "/cellar/") || strings.Contains(normalized, "/homebrew/") {
		return InstallMethodHomebrew
	}
	if normalized != "" {
		return InstallMethodDirect
	}

	return InstallMethodUnknown
}

func updateCommandFor(method InstallMethod) string {
	switch method {
	case InstallMethodNPM:
		return "npm i -g skill-organizer@latest"
	case InstallMethodHomebrew:
		return "brew upgrade skill-organizer"
	default:
		return "download the latest release from GitHub"
	}
}

func shouldSkipPeriodicCheck() bool {
	args := os.Args[1:]
	if len(args) == 0 {
		return true
	}

	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "--version" || arg == "-v" {
			return true
		}
	}

	switch args[0] {
	case "completion", "help", "self-update":
		return true
	default:
		return false
	}
}

func cachedVersionInfo(state configpkg.UpdateCache, currentVersion string, method InstallMethod, now time.Time) (VersionInfo, bool) {
	checkedAt, err := time.Parse(time.RFC3339, state.LastCheckedAt)
	if err != nil || now.Sub(checkedAt) >= checkInterval {
		return VersionInfo{}, false
	}
	if strings.TrimSpace(state.LatestVersion) == "" {
		return VersionInfo{}, false
	}

	return VersionInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   state.LatestVersion,
		LatestPageURL:   pageURLOrDefault(state.LatestPageURL),
		UpdateCommand:   updateCommandFor(method),
		InstallMethod:   method,
		UpdateAvailable: isRemoteNewer(currentVersion, state.LatestVersion),
	}, true
}

func fetchLatestRelease(ctx context.Context) (latestRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubLatestURL, nil)
	if err != nil {
		return latestRelease{}, fmt.Errorf("build latest release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return latestRelease{}, fmt.Errorf("request latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return latestRelease{}, fmt.Errorf("request latest release: unexpected status %s", resp.Status)
	}

	var payload releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return latestRelease{}, fmt.Errorf("decode latest release: %w", err)
	}

	version := strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	if version == "" {
		return latestRelease{}, errors.New("latest release tag is empty")
	}

	archiveName := archiveNameFor(version)
	tagName := strings.TrimSpace(payload.TagName)
	if tagName == "" {
		tagName = "v" + version
	}

	return latestRelease{
		version:  version,
		pageURL:  pageURLOrDefault(payload.HTMLURL),
		assetURL: fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", githubOwner, githubRepo, tagName, archiveName),
	}, nil
}

func archiveNameFor(version string) string {
	osName := map[string]string{
		"linux":   "Linux",
		"darwin":  "Darwin",
		"windows": "Windows",
	}[runtime.GOOS]
	archName := map[string]string{
		"amd64": "x86_64",
		"arm64": "arm64",
		"arm":   "arm",
	}[runtime.GOARCH]
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("skill-organizer_%s_%s_%s%s", version, osName, archName, ext)
}

func updateDirectBinary(ctx context.Context, latest latestRelease) error {
	_ = ctx
	_ = latest
	return fmt.Errorf("direct self-update is disabled until release integrity verification is anchored to an independent trust root; download the latest release from %s", latest.pageURL)
}

func runCommand(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", strings.Join(append([]string{name}, args...), " "), err)
	}
	return nil
}

func downloadToFile(ctx context.Context, url, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}

	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}

	return nil
}

func extractBinary(archivePath, destination string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractBinaryFromZIP(archivePath, destination)
	}
	return extractBinaryFromTarGz(archivePath, destination)
}

func extractBinaryFromTarGz(archivePath, destination string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("open gzip archive: %w", err)
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)
	binaryName := binaryNameForCurrentPlatform()
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || path.Base(header.Name) != binaryName {
			continue
		}

		extractedPath := filepath.Join(destination, binaryName)
		out, err := os.Create(extractedPath)
		if err != nil {
			return "", fmt.Errorf("create extracted binary: %w", err)
		}
		if _, err := io.Copy(out, tarReader); err != nil {
			out.Close()
			return "", fmt.Errorf("write extracted binary: %w", err)
		}
		if err := out.Close(); err != nil {
			return "", fmt.Errorf("close extracted binary: %w", err)
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(extractedPath, 0o755); err != nil {
				return "", fmt.Errorf("mark extracted binary executable: %w", err)
			}
		}
		return extractedPath, nil
	}

	return "", fmt.Errorf("binary %s not found in archive", binaryName)
}

func extractBinaryFromZIP(archivePath, destination string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("open zip archive: %w", err)
	}
	defer reader.Close()

	binaryName := binaryNameForCurrentPlatform()
	for _, file := range reader.File {
		if path.Base(file.Name) != binaryName {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("open zip entry: %w", err)
		}
		extractedPath := filepath.Join(destination, binaryName)
		out, err := os.Create(extractedPath)
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("create extracted binary: %w", err)
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return "", fmt.Errorf("write extracted binary: %w", err)
		}
		if err := rc.Close(); err != nil {
			out.Close()
			return "", fmt.Errorf("close zip entry: %w", err)
		}
		if err := out.Close(); err != nil {
			return "", fmt.Errorf("close extracted binary: %w", err)
		}
		return extractedPath, nil
	}

	return "", fmt.Errorf("binary %s not found in archive", binaryName)
}

func replaceExecutable(sourcePath, targetPath string) error {
	if runtime.GOOS == "windows" {
		backupPath := targetPath + ".old"
		_ = os.Remove(backupPath)
		if err := os.Rename(targetPath, backupPath); err != nil {
			return fmt.Errorf("move current executable out of the way: %w", err)
		}
		if err := os.Rename(sourcePath, targetPath); err != nil {
			return fmt.Errorf("replace executable: %w", err)
		}
		_ = os.Remove(backupPath)
		return nil
	}

	if err := os.Rename(sourcePath, targetPath); err == nil {
		return nil
	}

	input, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open replacement binary: %w", err)
	}
	defer input.Close()

	output, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("open target executable: %w", err)
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("overwrite target executable: %w", err)
	}
	return os.Chmod(targetPath, 0o755)
}

func binaryNameForCurrentPlatform() string {
	if runtime.GOOS == "windows" {
		return "skill-organizer.exe"
	}
	return "skill-organizer"
}

func commandForCurrentPlatform(defaultName, windowsName string) string {
	if runtime.GOOS == "windows" {
		return windowsName
	}
	return defaultName
}

func pageURLOrDefault(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultPageURL
	}
	return trimmed
}

func clearCachedUpdateState() error {
	cachePath, err := configpkg.CachePath()
	if err != nil {
		return err
	}
	return configpkg.ClearUpdateCache(cachePath)
}

func isRemoteNewer(currentVersion, latestVersion string) bool {
	current := parseSemver(currentVersion)
	latest := parseSemver(latestVersion)
	if current.valid && latest.valid {
		return compareSemver(current, latest) < 0
	}
	return strings.TrimSpace(currentVersion) != "" && strings.TrimSpace(latestVersion) != "" && strings.TrimSpace(currentVersion) != strings.TrimSpace(latestVersion)
}

type semver struct {
	major      int
	minor      int
	patch      int
	prerelease string
	valid      bool
}

func parseSemver(value string) semver {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if trimmed == "" {
		return semver{}
	}

	withNoBuild := strings.SplitN(trimmed, "+", 2)[0]
	coreAndPre := strings.SplitN(withNoBuild, "-", 2)
	coreParts := strings.Split(coreAndPre[0], ".")
	if len(coreParts) != 3 {
		return semver{}
	}

	major, err := strconv.Atoi(coreParts[0])
	if err != nil {
		return semver{}
	}
	minor, err := strconv.Atoi(coreParts[1])
	if err != nil {
		return semver{}
	}
	patch, err := strconv.Atoi(coreParts[2])
	if err != nil {
		return semver{}
	}

	prerelease := ""
	if len(coreAndPre) == 2 {
		prerelease = coreAndPre[1]
	}

	return semver{major: major, minor: minor, patch: patch, prerelease: prerelease, valid: true}
}

func compareSemver(left, right semver) int {
	if left.major != right.major {
		return compareInts(left.major, right.major)
	}
	if left.minor != right.minor {
		return compareInts(left.minor, right.minor)
	}
	if left.patch != right.patch {
		return compareInts(left.patch, right.patch)
	}
	if left.prerelease == right.prerelease {
		return 0
	}
	if left.prerelease == "" {
		return 1
	}
	if right.prerelease == "" {
		return -1
	}
	if left.prerelease < right.prerelease {
		return -1
	}
	if left.prerelease > right.prerelease {
		return 1
	}
	return 0
}

func compareInts(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}
