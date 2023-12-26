package tracing

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"go.opentelemetry.io/otel"
	otlpgrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otlphttp "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

const OtlpEndpointEnvVar = "OTEL_EXPORTER_OTLP_ENDPOINT"
const OtlpTracesEndpointEnvVar = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
const OtlpHeadersEnvVar = "OTEL_EXPORTER_OTLP_HEADERS"
const OtelTraceExporterEnvVar = "OTEL_TRACE_EXPORTER"

func Configure(ctx context.Context, appName string, version string) (func(ctx context.Context) error, error) {
	exporter, err := createExporter(ctx)
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(appName),
			semconv.ServiceVersionKey.String(version),
		)),
	)

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-signals
		fmt.Printf("Received %s, stopping\n", s)

		if err := tp.Shutdown(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		os.Exit(0)
	}()

	return tp.Shutdown, nil
}

func parseOtelEnvironmentHeaders(fromEnv string) (map[string]string, error) {

	headers := map[string]string{}

	if fromEnv == "" {
		return headers, nil
	}

	for _, pair := range strings.Split(fromEnv, ",") {
		index := strings.Index(pair, "=")
		if index == -1 {
			return nil, fmt.Errorf("unable to parse '%s' as a key=value pair, missing a '='", pair)
		}

		key := strings.TrimSpace(pair[0:index])
		val := strings.TrimSpace(pair[index+1:])

		headers[key] = val
	}

	return headers, nil
}

func createExporter(ctx context.Context) (sdktrace.SpanExporter, error) {

	exporterType := os.Getenv(OtelTraceExporterEnvVar)
	switch exporterType {
	case "stdout":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())

	case "stderr":
		return stdouttrace.New(stdouttrace.WithPrettyPrint(), stdouttrace.WithWriter(os.Stderr))
	}

	endpoint := "localhost:4317"
	if val := os.Getenv(OtlpTracesEndpointEnvVar); val != "" {
		endpoint = val
	} else if val := os.Getenv(OtlpEndpointEnvVar); val != "" {
		endpoint = val
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "http://") {

		opts := []otlphttp.Option{}

		hostAndPort := u.Host
		if u.Port() == "" {
			if u.Scheme == "https" {
				hostAndPort += ":443"
			} else {
				hostAndPort += ":80"
			}
		}
		opts = append(opts, otlphttp.WithEndpoint(hostAndPort))

		if u.Path == "" {
			u.Path = "/v1/traces"
		}
		opts = append(opts, otlphttp.WithURLPath(u.Path))

		if u.Scheme == "http" {
			opts = append(opts, otlphttp.WithInsecure())
		}

		headers, err := parseOtelEnvironmentHeaders(os.Getenv(OtlpHeadersEnvVar))
		if err != nil {
			return nil, err
		}
		opts = append(opts, otlphttp.WithHeaders(headers))

		return otlphttp.New(ctx, opts...)
	} else {
		opts := []otlpgrpc.Option{}

		opts = append(opts, otlpgrpc.WithEndpoint(endpoint))

		isLocal, err := isLoopbackAddress(endpoint)
		if err != nil {
			return nil, err
		}

		if isLocal {
			opts = append(opts, otlpgrpc.WithInsecure())
		}

		headers, err := parseOtelEnvironmentHeaders(os.Getenv(OtlpHeadersEnvVar))
		if err != nil {
			return nil, err
		}
		opts = append(opts, otlpgrpc.WithHeaders(headers))

		return otlpgrpc.New(ctx, opts...)
	}

}

func isLoopbackAddress(endpoint string) (bool, error) {
	hpRe := regexp.MustCompile(`^[\w.-]+:\d+$`)
	uriRe := regexp.MustCompile(`^(http|https)`)

	endpoint = strings.TrimSpace(endpoint)

	var hostname string
	if hpRe.MatchString(endpoint) {
		parts := strings.SplitN(endpoint, ":", 2)
		hostname = parts[0]
	} else if uriRe.MatchString(endpoint) {
		u, err := url.Parse(endpoint)
		if err != nil {
			return false, err
		}
		hostname = u.Hostname()
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return false, err
	}

	allAreLoopback := true
	for _, ip := range ips {
		if !ip.IsLoopback() {
			allAreLoopback = false
		}
	}

	return allAreLoopback, nil
}
