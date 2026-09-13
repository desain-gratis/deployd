ifneq (,$(wildcard .env))
include .env
export
endif


clean-wsl: 
	docker compose down -v # important for wsl
	sudo rm -rf ./tmp*           

clean:
	rm -rf ./tmp*
	
build:
	go test -v ./internal/... ./src/...
	CGO_ENABLED=0 GOOS=linux go build -o deployd cmd/deployd/*.go
	docker build --pull=false --network=host -t deployd .

run:
	docker compose up --force-recreate

build-user-profile:
	mkdir -p archive
	CGO_ENABLED=0 GOOS=linux go build -o ./archive/user-profile cmd/test/user-profile/*.go
	tar -czvf user-profile.tar.gz archive

build-website:
	tar -czvf deployd-website.tar.gz www

configure: build-user-profile build-website
	go run ./cmd/test/configure/*.go

submit-job:
	curl -X POST -H 'X-Namespace: *' 'http://localhost:9401/deployd/submit-job' -d'@submit-sample.json' | jq

submit-job-website:
	curl -X POST -H 'X-Namespace: *' 'http://localhost:9401/job/configure-website/submit' -d'@configure-website.json'  | jq

submit-job-website-from-url:
	curl -X POST -H 'X-Namespace: *' 'http://localhost:9401/job/configure-website/submit' -d'@configure-website-from-url.json'  | jq

submit-job-mb:
	curl -X POST -H 'X-Namespace: *' 'http://mb1:9600/deployd/submit-job' -d'@submit-sample-mb.json' | jq

get-job:
	curl -X GET -H 'X-Namespace: *' 'http://localhost:9401/deployd/job' | jq

get-job-website:
	curl -X GET -H 'X-Namespace: *' 'http://localhost:9401/job/configure-website' | jq

get-job-mb:
	curl -X GET -H 'X-Namespace: *' 'http://mb1:9600/deployd/job' | jq

test: configure submit-job get-job submit-job-website submit-job-website-from-url get-job-website 

tail:
	curl -X GET -H 'X-Namespace: *' 'http://localhost:9401/deployd/job/tail'
