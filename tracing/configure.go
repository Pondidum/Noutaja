package tracing

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.opentelemetry.io/otel"
	otlpgrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otlphttp "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

const OtlpEndpointEnvVar = "OTEL_EXPORTER_OTLP_ENDPOINT"
const OtlpTracesEndpointEnvVar = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
const OtelTraceExporterEnvVar = "OTEL_TRACES_EXPORTER"

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

func createExporter(ctx context.Context) (tracesdk.SpanExporter, error) {

	exporterType := os.Getenv(OtelTraceExporterEnvVar)
	switch exporterType {
	case "stdout":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())

	case "stderr":
		return stdouttrace.New(stdouttrace.WithPrettyPrint(), stdouttrace.WithWriter(os.Stderr))
	}

	endpoint := ""
	if val := os.Getenv(OtlpTracesEndpointEnvVar); val != "" {
		endpoint = val
	} else if val := os.Getenv(OtlpEndpointEnvVar); val != "" {
		endpoint = val
	}

	if strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "http://") {
		return otlphttp.New(ctx)
	} else {
		return otlpgrpc.New(ctx)
	}

}
