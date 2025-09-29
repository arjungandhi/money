package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v52/github"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/version"
)

var Update = &Z.Cmd{
	Name:     "update",
	Summary:  "Check for and display information about available updates",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		fmt.Printf("Current version: %s\n", version.Version)

		// If version is "dev", skip update check
		if version.Version == "dev" {
			fmt.Println("Development version detected. Cannot check for updates.")
			fmt.Println("Please use a release build or manually check: https://github.com/arjungandhi/money/releases")
			return nil
		}

		fmt.Println("Checking for updates...")

		client := github.NewClient(nil)
		release, _, err := client.Repositories.GetLatestRelease(context.Background(), "arjungandhi", "money")
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		latestVersion := strings.TrimPrefix(release.GetTagName(), "v")
		currentVersion := strings.TrimPrefix(version.Version, "v")

		if latestVersion == currentVersion {
			fmt.Printf("✅ You are running the latest version (%s)\n", version.Version)
		} else {
			fmt.Printf("🆕 New version available: %s (current: %s)\n", latestVersion, currentVersion)
			fmt.Printf("📝 Release notes: %s\n", release.GetHTMLURL())
			fmt.Printf("⬇️  Download: %s\n", release.GetHTMLURL())
			
			if release.GetBody() != "" {
				fmt.Printf("\nWhat's new:\n%s\n", release.GetBody())
			}
		}

		return nil
	},
}
