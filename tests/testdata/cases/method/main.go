// The service resource comes from a method on a builder value.
package main

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type builder struct{ name string }

func (b builder) build() *resource.Resource {
	return resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(b.name))
}

func main() {
	b := builder{name: "case"}
	res, err := resource.Merge(resource.Default(), b.build())
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
