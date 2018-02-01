build:
	@go build -o ./bin/neo-go ./cli/main.go

deps:
	@glide install

test:
	@go test $(glide nv) -cover