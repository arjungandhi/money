package cli

import (
	"fmt"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/version"
)

var Version = &Z.Cmd{
	Name:        "version",
	Aliases:     []string{"v", "ver"},
	Summary:     "Display the current version of the money CLI",
	Description: `Shows the build version (defaults to "dev" for development builds).`,
	Commands:    []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		fmt.Println(version.Version)
		return nil
	},
}