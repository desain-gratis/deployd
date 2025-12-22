include .env

clean-wsl: 
	docker compose down -v # important for wsl
	sudo rm -rf ./tmp           

clean:
	rm -rf ./tmp
	
build:
	CGO_ENABLED=0 GOOS=linux go build -o deployd cmd/deployd/*.go
	docker build . -t deployd

run:
	docker compose up
