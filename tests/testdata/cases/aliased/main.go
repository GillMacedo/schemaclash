// sdk/resource imported under another name.
package main

import (
	"fmt"

	sdkres "go.opentelemetry.io/otel/sdk/resource"
	conv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
	res, err := sdkres.Merge(sdkres.Default(), sdkres.NewWithAttributes(conv.SchemaURL, conv.ServiceName("case")))
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}
