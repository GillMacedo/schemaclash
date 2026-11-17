.PHONY: test lint demo

test:
	test -z "$$(gofmt -l plugin tests examples/shop/cmd examples/shop/internal)"
	go vet ./...
	cd examples/shop && go vet ./...
	go test -race ./...

lint:
	claude plugin validate ./plugin --strict
	claude plugin validate . --strict

# The full example exits 1 because it has conflicts on purpose.
demo:
	go run ./plugin/scripts --dir examples/shop || test $$? -eq 1
	go run ./plugin/scripts --dir examples/shop ./cmd/worker ./cmd/billing
