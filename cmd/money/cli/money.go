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
		Version,
		Update,
		Init,
		Fetch,
		Balance,
		Accounts,
		Categories,
		Property,
		Budget,
		Transactions,
	},
}
