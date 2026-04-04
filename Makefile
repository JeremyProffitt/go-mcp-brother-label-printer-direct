.PHONY: build clean deploy local tidy test

build:
	cd cmd/lambda && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap .

clean:
	rm -f cmd/lambda/bootstrap
	rm -rf .aws-sam

deploy: build
	sam deploy \
		--template-file template.yaml \
		--stack-name mcp-brother-label-printer-direct-stack \
		--resolve-s3 \
		--capabilities CAPABILITY_IAM \
		--no-confirm-changeset

local:
	go run cmd/lambda/main.go

test:
	go test -v -race ./...

tidy:
	go mod tidy
