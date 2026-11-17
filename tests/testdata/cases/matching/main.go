// The same pattern, with the semconv version the SDK uses.
package main

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("case")))
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
