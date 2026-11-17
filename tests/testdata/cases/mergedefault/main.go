// The OpenTelemetry Go getting-started pattern with an older semconv.
package main

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("case")))
	report(res, err)
}

func report(res *resource.Resource, err error) {
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
