package cli

import (
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

var Cmd = &Z.Cmd{
	Name:    "money",
	Summary: "Personal finance management CLI",
	Commands: []*Z.Cmd{
		help.Cmd,
		Init,
		Fetch,
		Balance,
		Accounts,
		Property,
		Budget,
		Transactions,
	},
}
