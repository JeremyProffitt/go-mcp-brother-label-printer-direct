package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go-mcp-brother-label-printer-direct/internal/telemetry"
)

func OTelTracing() fiber.Handler {
	tracer := telemetry.Tracer("mcp-brother-label-printer-direct")

	return func(c *fiber.Ctx) error {
		metrics := telemetry.GetMetrics()

		ctx, span := tracer.Start(c.Context(), fmt.Sprintf("%s %s", string(c.Request().Header.Method()), c.Path()),
			trace.WithAttributes(
				attribute.String("http.method", string(c.Request().Header.Method())),
				attribute.String("http.url", c.OriginalURL()),
				attribute.String("http.target", c.Path()),
				attribute.String("net.peer.ip", c.IP()),
			),
		)
		defer span.End()

		c.SetUserContext(ctx)

		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		status := c.Response().StatusCode()
		span.SetAttributes(attribute.Int("http.status_code", status))

		if metrics != nil {
			metrics.RequestCount.Add(context.Background(), 1,
				telemetry.WithServerAttr("printer"),
			)
			metrics.Latency.Record(context.Background(), float64(duration.Milliseconds()),
				telemetry.WithServerAttr("printer"),
			)
			if status >= 400 {
				metrics.RequestErrors.Add(context.Background(), 1,
					telemetry.WithServerAttr("printer"),
				)
			}
		}

		if err != nil {
			span.RecordError(err)
		}

		return err
	}
}
