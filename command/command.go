package command

import (
	"context"
	"fmt"
	"noutaja/tracing"
	"os"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
)

type CommandDefinition interface {
	Synopsis() string
	Flags() *pflag.FlagSet
	Execute(ctx context.Context, args []string) error
}

func NewCommand(definition CommandDefinition) func() (cli.Command, error) {
	return func() (cli.Command, error) {
		return &command{definition}, nil
	}
}

type command struct {
	CommandDefinition
}

func (c *command) Help() string {
	sb := strings.Builder{}

	sb.WriteString(c.Synopsis())
	sb.WriteString("\n\n")

	sb.WriteString("Flags:\n\n")

	sb.WriteString(c.Flags().FlagUsagesWrapped(80))

	return sb.String()
}

func (c *command) Run(args []string) int {
	ctx := context.Background()
	shutdown, err := tracing.Configure(ctx, "noutaja", "0.0.1")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	defer shutdown(ctx)

	tr := otel.Tracer("noutaja")
	ctx, span := tr.Start(ctx, "main")
	defer span.End()

	flags := c.Flags()

	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	if err := c.Execute(ctx, flags.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	return 0
}
