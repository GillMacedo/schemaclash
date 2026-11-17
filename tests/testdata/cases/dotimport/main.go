// semconv imported with a dot.
package main

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	. "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(SchemaURL, ServiceName("case")))
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
