// The schema URL is passed to a helper package that does the merge.
package main

import (
	"fmt"

	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"example.com/shop/cmd/param/helper"
)

func main() {
	res, err := helper.Build(semconv.SchemaURL, "case")
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
