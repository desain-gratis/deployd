build:
	GOOS=linux GOARCH=amd64 go build -o deployd cmd/deployd/*.go
run:
	go run cmd/deployd/*.go
