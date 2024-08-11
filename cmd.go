// zet Command Line tool
package money

import (
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

// rootCmd is the main command for the money command line tool
// its just holds all the other useful commands
var Cmd = &Z.Cmd{
	Name:    "money",
	Summary: "money is a command line tool for managing my money"
	Commands: []*Z.Cmd{
		help.Cmd,
	},
}
