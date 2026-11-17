// Old semconv attributes in a resource with no schema URL.
package main

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName("case")))
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
