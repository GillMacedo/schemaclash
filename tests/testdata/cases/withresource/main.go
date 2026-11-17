// An old schema URL passed straight to the tracer provider, which merges it
// only with the environment's resource, which has no schema URL.
package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

func main() {
	var otelErr error
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) { otelErr = err }))
	res := resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("case"))
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	defer tp.Shutdown(context.Background())
	if otelErr != nil {
		fmt.Println("conflict:", otelErr)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
