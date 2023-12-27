package server

import (
	"context"
	"net/http"
	"noutaja/cache"
	"noutaja/tracing"
	"os"
	"path"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
)

var tr = otel.Tracer("server")

type ServerCommand struct {
	addr  string
	cache string
}

func NewServerCommand() *ServerCommand {
	return &ServerCommand{
		addr:  "localhost:5959",
		cache: "",
	}
}

func (c *ServerCommand) Synopsis() string {
	return "run the server"
}

func (c *ServerCommand) Flags() *pflag.FlagSet {

	flags := pflag.NewFlagSet("server", pflag.ContinueOnError)
	flags.StringVar(&c.addr, "addr", "localhost:5959", "")
	flags.StringVar(&c.cache, "cache-dir", "", "")

	return flags
}

func (cmd *ServerCommand) Execute(ctx context.Context, args []string) error {
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
		attribute.String("flag.addr", cmd.addr),
	)

	if err := os.MkdirAll(cmd.cache, 0700); err != nil {
		return tracing.Error(span, err)
	}

	app := fiber.New(fiber.Config{})
	app.Use(otelfiber.Middleware())

	app.Get("/fetch", func(c *fiber.Ctx) error {
		ctx, span := tr.Start(c.UserContext(), "fetch")
		defer span.End()

		dto := &FetchDto{
			Url:     c.Query("url", ""),
			Refetch: c.QueryBool("refetch", false),
		}

		if dto.Url == "" {
			c.Status(http.StatusBadRequest)
			return nil
		}

		span.SetAttributes(
			attribute.String("query.url", dto.Url),
			attribute.Bool("query.refetch", dto.Refetch),
		)

		location, err := cache.Get(ctx, cache.GetArgs{
			CachePath: cmd.cache,
			Url:       dto.Url,
			Refetch:   dto.Refetch,
		})
		if err != nil {
			return tracing.Error(span, err)
		}

		filename := path.Base(dto.Url)
		span.SetAttributes(attribute.String("response.filename", filename))

		if err := c.Download(location, filename); err != nil {
			return tracing.Error(span, err)
		}

		span.SetAttributes(attribute.Bool("response.sent", true))

		return nil
	})

	return app.Listen(cmd.addr)
}

type FetchDto struct {
	Url     string
	Refetch bool
}
