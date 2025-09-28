package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/arjungandhi/money/pkg/version"
	"github.com/google/go-github/v52/github"
)

// CheckForUpdate checks for a new version of the CLI
func CheckForUpdate() (bool, string, error) {
	release, err := GetLatestRelease()
	if err != nil {
		return false, "", err
	}

	if release.TagName == nil {
		return false, "", errors.New("no tag name found in latest release")
	}

	currentVersion := version.Version
	latestVersion := *release.TagName

	if currentVersion == latestVersion {
		return false, latestVersion, nil
	}

	return true, latestVersion, nil
}

// GetLatestRelease gets the latest release from GitHub
func GetLatestRelease() (*github.RepositoryRelease, error) {
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), "arjungandhi", "money")
	if err != nil {
		return nil, err
	}

	return release, nil
}

// UpdateBinary downloads and replaces the current binary with the latest version
func UpdateBinary() error {
	release, err := GetLatestRelease()
	if err != nil {
		return err
	}

	// Find the current binary path
	path, err := exec.LookPath("money")
	if err != nil {
		return fmt.Errorf("could not find money binary in PATH: %w", err)
	}

	// Determine the asset name based on platform
	assetName := fmt.Sprintf("money-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	// Find the correct asset
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name != nil && *asset.Name == assetName {
			downloadURL = *asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary found for platform %s-%s", runtime.GOOS, runtime.GOARCH)
	}

	// Download the binary
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download binary: HTTP %d", resp.StatusCode)
	}

	// Create a temporary file
	tmpDir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(tmpDir, "money-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Copy the downloaded binary to the temporary file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	// Close the temporary file
	err = tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Make the temporary file executable
	err = os.Chmod(tmpFile.Name(), 0755)
	if err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Replace the current binary
	err = os.Rename(tmpFile.Name(), path)
	if err != nil {
		return fmt.Errorf("failed to replace binary. Please ensure you have write permissions to %s: %w", path, err)
	}

	return nil
}