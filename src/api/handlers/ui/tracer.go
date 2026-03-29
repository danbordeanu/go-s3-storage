package ui

import (
	"s3-storage/configuration"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// tracer init
var tracer = otel.Tracer(configuration.OTName, trace.WithInstrumentationVersion(configuration.OTVersion), trace.WithSchemaURL(configuration.OTSchema))
