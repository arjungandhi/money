package cli

import (
	"fmt"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/version"
)

var Version = &Z.Cmd{
	Name:     "version",
	Aliases:  []string{"v"},
	Summary:  "Show money CLI version information",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		fmt.Printf("money version %s\n", version.Version)
		return nil
	},
}
