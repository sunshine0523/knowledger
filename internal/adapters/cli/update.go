package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const githubRepo = "sunshine0523/claude-knowledger"

type UpdateRunner func(version string, runClaudeInstall ClaudeInstallRunner, out, errOut io.Writer) error

func newUpdateCommand(version string, runClaudeInstall ClaudeInstallRunner, runUpdate UpdateRunner) *cobra.Command {
	if runUpdate == nil {
		runUpdate = DefaultUpdateRunner
	}
	var checkOnly bool
	var skipPlugin bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update knowledger to the latest version",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()
			if checkOnly {
				return checkLatestVersion(version, out)
			}
			install := runClaudeInstall
			if skipPlugin {
				install = func(out, errOut io.Writer) error { return nil }
			}
			return runUpdate(version, install, out, errOut)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check for updates without installing")
	cmd.Flags().BoolVar(&skipPlugin, "skip-plugin", false, "Skip reinstalling the Claude Code plugin after update")
	return cmd
}

func DefaultUpdateRunner(version string, runClaudeInstall ClaudeInstallRunner, out, errOut io.Writer) error {
	latest, assetURL, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if version == latest {
		fmt.Fprintf(out, "Already up to date (%s)\n", version)
		return nil
	}
	fmt.Fprintf(out, "Updating %s → %s\n", version, latest)

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	if err := downloadAndReplace(assetURL, execPath, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "Binary updated to %s\n", latest)

	if runClaudeInstall != nil {
		fmt.Fprintln(out, "Reinstalling Claude Code plugin...")
		if err := runClaudeInstall(out, errOut); err != nil {
			fmt.Fprintf(errOut, "Warning: plugin reinstall failed: %v\n", err)
			fmt.Fprintln(errOut, "Run 'knowledger install --claude' manually to complete the update.")
		}
	}
	return nil
}

func checkLatestVersion(current string, out io.Writer) error {
	latest, _, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}
	if current == latest {
		fmt.Fprintf(out, "Up to date (%s)\n", current)
	} else {
		fmt.Fprintf(out, "Update available: %s → %s\n", current, latest)
		fmt.Fprintf(out, "Run 'knowledger update' to install.\n")
	}
	return nil
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func fetchLatestRelease() (version, assetURL string, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", err
	}
	suffix := assetSuffix()
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, suffix) {
			return rel.TagName, a.BrowserDownloadURL, nil
		}
	}
	return "", "", fmt.Errorf("no asset matching %q found in release %s", suffix, rel.TagName)
}

// assetSuffix returns the platform-specific suffix of the goreleaser archive,
// e.g. "_linux_amd64.tar.gz" or "_windows_amd64.zip". Matching by suffix keeps
// this resilient to the goreleaser {ProjectName}_{Version}_ prefix changing.
func assetSuffix() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("_%s_%s.%s", goos, goarch, ext)
}

func downloadAndReplace(assetURL, execPath string, out io.Writer) error {
	fmt.Fprintf(out, "Downloading %s...\n", assetURL)
	resp, err := http.Get(assetURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading download: %w", err)
	}

	var binaryData []byte
	if strings.HasSuffix(assetURL, ".zip") {
		binaryData, err = extractFromZip(data, "knowledger.exe")
	} else {
		binaryData, err = extractFromTarGz(data, "knowledger")
	}
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	// Atomic replace: write temp file, then rename over existing binary.
	tmpPath := execPath + ".new"
	if err := os.WriteFile(tmpPath, binaryData, 0o755); err != nil {
		return fmt.Errorf("writing new binary: %w", err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replacing binary: %w", err)
	}
	return nil
}

func extractFromTarGz(data []byte, binaryName string) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == binaryName {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%q not found in archive", binaryName)
}

func extractFromZip(data []byte, binaryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%q not found in zip", binaryName)
}
