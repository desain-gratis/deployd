include .env

clean-wsl: 
	docker compose down -v # important for wsl
	sudo rm -rf ./tmp*           

clean:
	rm -rf ./tmp*
	
build:
	CGO_ENABLED=0 GOOS=linux go build -o deployd cmd/deployd/*.go
	docker build . -t deployd

run:
	docker compose up

build-user-profile:
	mkdir -p archive
	CGO_ENABLED=0 GOOS=linux go build -o ./archive/user-profile cmd/test/user-profile/*.go
	tar -czvf user-profile.tar.gz archive

configure:
	go run ./cmd/test/configure/*.go

submit-job:
	curl -X POST -H 'X-Namespace: *' 'http://localhost:9401/deployd/job/submit' -d'@submit-sample.json' | jq

get-job:
	curl -X GET -H 'X-Namespace: *' 'http://localhost:9401/deployd/job' | jq

test: configure build-user-profile configure submit-job get-job

tail:
	curl -X GET -H 'X-Namespace: *' 'http://localhost:9401/deployd/job/tail'
