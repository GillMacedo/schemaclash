// Command api starts the shop's HTTP API with tracing.
package main

import (
	"context"
	"fmt"
	"log"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"example.com/shop/internal/telemetry"
)

func main() {
	res, err := telemetry.NewResource("shop-api", "1.4.2")
	if err != nil {
		log.Fatalf("api: telemetry: %v", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	defer tp.Shutdown(context.Background())
	fmt.Println("api: started, resource schema", res.SchemaURL())
}
