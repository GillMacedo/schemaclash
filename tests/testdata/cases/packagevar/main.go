// The service resource is a package-level variable.
package main

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var service = resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("case"))

func main() {
	res, err := resource.Merge(resource.Default(), service)
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
