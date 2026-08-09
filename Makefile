.PHONY: install update linux test all

install:
	go install ./...

update:
	go mod tidy

linux:
	env GOOS=linux GOARCH=amd64 go build -o gota ./cmd/gota

test:
	go test ./...
