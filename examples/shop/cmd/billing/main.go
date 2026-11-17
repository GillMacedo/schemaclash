// Command billing sends invoices. It still uses semconv v1.20.0, but it
// never merges its resource with one that has a schema URL, so it starts.
package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

func main() {
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Fatalf("billing: otel: %v", err)
	}))
	res := resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("shop-billing"))
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("example.com/shop/cmd/billing")
	_, span := tracer.Start(context.Background(), "send-invoices")
	span.End()
	fmt.Println("billing: started, resource schema", res.SchemaURL())
}
