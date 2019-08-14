build:
	@go build -o bin/neo-go main.go

run: build
	@./bin/neo-go

test:
	@go test -v --cover ./... 

clean:
	@rm -rf bin
