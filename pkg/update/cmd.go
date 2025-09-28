package update

import (
	"fmt"

	"github.com/arjungandhi/money/pkg/version"
	Z "github.com/rwxrob/bonzai/z"
)

// Cmd is the update command
var Cmd = &Z.Cmd{
	Name:    "update",
	Summary: "Update the money CLI to the latest version",
	Call: func(cmd *Z.Cmd, args ...string) error {
		// Check if update is available
		updateAvailable, latestVersion, err := CheckForUpdate()
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		if !updateAvailable {
			fmt.Println("You are already on the latest version:", version.Version)
			return nil
		}

		fmt.Printf("Update available: %s -> %s\n", version.Version, latestVersion)
		fmt.Println("Downloading and installing update...")

		err = UpdateBinary()
		if err != nil {
			return fmt.Errorf("failed to update binary: %w", err)
		}

		fmt.Printf("Successfully updated to version %s!\n", latestVersion)
		fmt.Println("Run 'money version' to verify the update.")
		return nil
	},
}