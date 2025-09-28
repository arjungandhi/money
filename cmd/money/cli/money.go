package cli

import (
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
	"github.com/arjungandhi/money/pkg/version"
	"github.com/arjungandhi/money/pkg/update"
)

var Cmd = &Z.Cmd{
	Name:    "money",
	Summary: "Personal finance management CLI",
	Commands: []*Z.Cmd{
		help.Cmd,
		version.Cmd,
		update.Cmd,
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
