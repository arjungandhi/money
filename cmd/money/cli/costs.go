package cli

import (
	"fmt"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

var Costs = &Z.Cmd{
	Name:     "costs",
	Summary:  "Show breakdown of costs by category for time period",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		// TODO: Implement costs logic
		fmt.Println("TODO: Implement costs command")
		return nil
	},
}
