package config

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/plaid/plaid-go/v27/plaid"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

// Cmd is the root command for the money command line tool
var Cmd = &Z.Cmd{
	Name:    "config",
	Summary: "manage the money cli config",
	Commands: []*Z.Cmd{
		help.Cmd,
		initCmd,
		echoCmd,
	},
}

// InitCmd is the command for initializing the money command line tool
var initCmd = &Z.Cmd{
	Name:    "init",
	Summary: "initializes the money command line tool",
	Commands: []*Z.Cmd{
		help.Cmd,
	},
	Call: func(_ *Z.Cmd, _ ...string) error {
		// Ask the user for their plaid credentials
		questions := []*survey.Question{
			{
				Name:     "clientID",
				Prompt:   &survey.Input{Message: "What is your Plaid client ID?"},
				Validate: survey.Required,
			},
			{
				Name:     "secret",
				Prompt:   &survey.Input{Message: "What is your Plaid secret?"},
				Validate: survey.Required,
			},
			{
				Name: "environment",
				Prompt: &survey.Select{
					Message: "What environment are you using?",
					Options: []string{
						"sandbox",
						"production",
					},
				},
			},
		}

		answers := struct {
			ClientID    string
			Secret      string
			Environment string
		}{}

		err := survey.Ask(questions, &answers)
		if err != nil {
			return err
		}

		// Create the config struct
		creds := Config{
			ClientID:    answers.ClientID,
			Secret:      answers.Secret,
			Environment: plaid.Environment(answers.Environment),
		}

		// Save the config
		err = creds.SaveConfig()
		if err != nil {
			return err
		}

		return nil
	},
}

// echo Cmd echos the currently set plaid credentials to the terminal
var echoCmd = &Z.Cmd{
	Name:    "echo",
	Summary: "echo the current plaid credentials",
	Commands: []*Z.Cmd{
		help.Cmd,
	},
	Call: func(_ *Z.Cmd, _ ...string) error {
		creds, err := LoadConfig()
		if err != nil {
			return err
		}

		fmt.Println("Client ID:", creds.ClientID)
		fmt.Println("Secret:", creds.Secret)
		fmt.Println("Environment:", creds.Environment)

		return nil
	},
}
