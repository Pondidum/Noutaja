package cache

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"noutaja/tracing"
	"os"
	"path"

	"github.com/hashicorp/go-getter"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tr = otel.Tracer("download")

type GetArgs struct {
	CachePath string
	Url       string
	Refetch   bool
}

func Get(ctx context.Context, args GetArgs) (string, error) {
	ctx, span := tr.Start(ctx, "download")
	defer span.End()

	span.SetAttributes(
		attribute.String("args.cache_path", args.CachePath),
		attribute.String("args.url", args.Url),
		attribute.Bool("args.refetch", args.Refetch),
	)

	hash := md5.Sum([]byte(args.Url))
	key := base64.RawStdEncoding.EncodeToString(hash[:])
	contentPath := path.Join(args.CachePath, key)

	_, err := os.Stat(contentPath)
	exists := err == nil || !errors.Is(err, os.ErrNotExist)

	if err != nil {
		// in case there is something interesting happening log the error, but don't fail the request
		span.RecordError(err)
	}

	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.path", contentPath),
		attribute.Bool("cache.hit", exists),
	)

	if !exists || args.Refetch {
		span.SetAttributes(attribute.Bool("get.remote", true))

		err := getter.GetFile(contentPath, args.Url, getter.WithContext(ctx))
		if err != nil {
			return "", tracing.Error(span, err)
		}

		span.SetAttributes(attribute.Bool("get.success", true))
	}

	return contentPath, nil
}
