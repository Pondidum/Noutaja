package main

import (
	"fmt"
	"noutaja/command"
	"noutaja/command/server"
	"os"

	"github.com/mitchellh/cli"
)

func main() {

	commands := map[string]cli.CommandFactory{
		"server": command.NewCommand(server.NewServerCommand()),
	}

	cli := &cli.CLI{
		Name:                       "noutaja",
		Args:                       os.Args[1:],
		Commands:                   commands,
		Autocomplete:               true,
		AutocompleteNoDefaultFlags: false,
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
	}

	os.Exit(exitCode)
}
