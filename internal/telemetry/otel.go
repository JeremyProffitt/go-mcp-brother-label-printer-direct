package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

type Metrics struct {
	RequestCount  metric.Int64Counter
	RequestErrors metric.Int64Counter
	Latency       metric.Float64Histogram
	TokensIssued  metric.Int64Counter
}

var globalMetrics *Metrics

func GetMetrics() *Metrics {
	return globalMetrics
}

func Init(ctx context.Context, serviceName, endpoint string) (shutdown func(context.Context) error, err error) {
	if endpoint == "" {
		slog.Info("OTel endpoint not configured, telemetry disabled")
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		slog.Warn("failed to create OTel resource", "error", err)
		return func(context.Context) error { return nil }, nil
	}

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(stripScheme(endpoint)),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithTimeout(3*time.Second),
	)
	if err != nil {
		slog.Warn("failed to create trace exporter", "error", err)
		return func(context.Context) error { return nil }, nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(128),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(stripScheme(endpoint)),
		otlpmetrichttp.WithInsecure(),
		otlpmetrichttp.WithTimeout(3*time.Second),
	)
	if err != nil {
		slog.Warn("failed to create metric exporter", "error", err)
		return tp.Shutdown, nil
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(30*time.Second),
		)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	meter := mp.Meter(serviceName)
	globalMetrics = &Metrics{}
	globalMetrics.RequestCount, _ = meter.Int64Counter("mcp.printer.requests",
		metric.WithDescription("Total MCP printer requests"),
	)
	globalMetrics.RequestErrors, _ = meter.Int64Counter("mcp.printer.errors",
		metric.WithDescription("Total MCP printer errors"),
	)
	globalMetrics.Latency, _ = meter.Float64Histogram("mcp.printer.latency_ms",
		metric.WithDescription("MCP printer request latency in milliseconds"),
	)
	globalMetrics.TokensIssued, _ = meter.Int64Counter("mcp.oauth.tokens_issued",
		metric.WithDescription("Total OAuth tokens issued"),
	)

	slog.Info("OpenTelemetry initialized", "endpoint", endpoint, "service", serviceName)

	return func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			slog.Warn("trace provider shutdown error", "error", err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			slog.Warn("metric provider shutdown error", "error", err)
		}
		return nil
	}, nil
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func WithServerAttr(name string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("mcp.server", name))
}

func stripScheme(url string) string {
	if len(url) > 8 && url[:8] == "https://" {
		return url[8:]
	}
	if len(url) > 7 && url[:7] == "http://" {
		return url[7:]
	}
	return url
}
