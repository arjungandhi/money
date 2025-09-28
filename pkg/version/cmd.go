package version

import (
	"fmt"

	Z "github.com/rwxrob/bonzai/z"
)

// Cmd is the version command
var Cmd = &Z.Cmd{
	Name:    "version",
	Summary: "Print the version of the money CLI",
	Call: func(cmd *Z.Cmd, args ...string) error {
		fmt.Println(Version)
		return nil
	},
}