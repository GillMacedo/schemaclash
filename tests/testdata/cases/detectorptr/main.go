// A local detector, passed as a pointer, next to a built-in one.
package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type region struct{}

func (*region) Detect(context.Context) (*resource.Resource, error) {
	return resource.NewWithAttributes(semconv.SchemaURL, semconv.CloudRegion("eu-west-1")), nil
}

func main() {
	res, err := resource.New(context.Background(), resource.WithDetectors(&region{}), resource.WithHost())
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
