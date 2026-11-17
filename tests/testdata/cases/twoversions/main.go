// Two semconv versions, neither of them the SDK's.
package main

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	old "go.opentelemetry.io/otel/semconv/v1.20.0"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
	res, err := resource.Merge(
		resource.NewWithAttributes(old.SchemaURL, old.ServiceName("case")),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceVersion("1.0.0")),
	)
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
