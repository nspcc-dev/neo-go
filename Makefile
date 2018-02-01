build:
	@go build -o neo-go cli/main.go

deps:
	@glide install

test:
	@go test $(glide nv) -cover