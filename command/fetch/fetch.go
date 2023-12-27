package fetch

import (
	"context"
	"io"
	"noutaja/cache"
	"noutaja/tracing"
	"os"
	"path"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tr = otel.Tracer("fetch")

type FetchCommand struct {
	cache   string
	refetch bool
	output  string
}

func NewFetchCommand() *FetchCommand {
	return &FetchCommand{
		cache:   "",
		refetch: false,
	}
}

func (c *FetchCommand) Synopsis() string {
	return "fetch an artifact"
}

func (c *FetchCommand) Flags() *pflag.FlagSet {

	flags := pflag.NewFlagSet("server", pflag.ContinueOnError)
	flags.StringVar(&c.cache, "cache-dir", "", "")
	flags.BoolVar(&c.refetch, "refetch", false, "")
	flags.StringVar(&c.output, "output", "", "")

	return flags
}

func (cmd *FetchCommand) Execute(ctx context.Context, args []string) error {
	ctx, span := tr.Start(ctx, "server")
	defer span.End()

	if cmd.cache == "" {
		path, err := os.MkdirTemp("", "noutaja-cache")
		if err != nil {
			return tracing.Error(span, err)
		}

		cmd.cache = path
	}

	span.SetAttributes(
		attribute.String("flag.cache-dir", cmd.cache),
		attribute.Bool("flag.refetch", cmd.refetch),
	)

	if err := os.MkdirAll(cmd.cache, 0700); err != nil {
		return tracing.Error(span, err)
	}

	if len(args) != 1 || args[0] == "" {
		return tracing.Errorf(span, "this command takes exactly 1 argument: url-ish")
	}

	location, err := cache.Get(ctx, cache.GetArgs{
		CachePath: cmd.cache,
		Url:       args[0],
		Refetch:   cmd.refetch,
	})
	if err != nil {
		return tracing.Error(span, err)
	}

	src, err := os.Open(location)
	if err != nil {
		return tracing.Error(span, err)
	}
	defer src.Close()

	dstDir := cmd.output
	if dstDir == "" {
		exe, err := os.Executable()
		if err != nil {
			return tracing.Error(span, err)
		}
		dstDir = path.Dir(exe)
	}

	filename := path.Base(args[0])
	dst, err := os.Create(path.Join(dstDir, filename))
	if err != nil {
		return tracing.Error(span, err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return tracing.Error(span, err)
	}

	return nil
}
