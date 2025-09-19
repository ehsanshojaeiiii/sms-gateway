package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/zap"
)

func SetupOpenTelemetry(serviceName string, logger *zap.Logger) (func(), error) {
	// Resource describes the service
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Set up Prometheus metrics exporter
	metricExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	// Set up metric provider
	metricProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metricExporter),
	)

	otel.SetMeterProvider(metricProvider)

	logger.Info("OpenTelemetry initialized",
		zap.String("service", serviceName))

	// Return cleanup function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := metricProvider.Shutdown(ctx); err != nil {
			logger.Error("error shutting down OpenTelemetry", zap.Error(err))
		}
	}, nil
}
