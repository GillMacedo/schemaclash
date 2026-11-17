// An old schema URL used for a tracer's scope, next to a resource built
// without one. Scope and resource schema URLs are never merged.
package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName("case")))
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("case", trace.WithSchemaURL(semconv.SchemaURL))
	_, span := tracer.Start(context.Background(), "start")
	span.End()
	fmt.Println("ok", res.SchemaURL())
}
