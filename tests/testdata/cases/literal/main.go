// A schema URL written as a string.
package main

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

func main() {
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes("https://opentelemetry.io/schemas/1.4.0", attribute.String("service.name", "case")))
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
