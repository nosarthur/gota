.PHONY=update install linux test all

install:
	go install ./...

update:
	go mod tidy

linux:
	env GOOS=linux GOARCH=amd64 go build -o gota cmd/gota/main.go

test:
	go test ./...tests
