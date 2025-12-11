build:
	GOOS=linux GOARCH=amd64 go build -o deployd cmd/deployd/*.go
	GOOS=linux GOARCH=amd64 go build -o artifactd cmd/artifactd/*.go
run:
	go run cmd/deployd/*.go
