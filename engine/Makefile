githash = $(shell git rev-parse --short HEAD)
ldflags = -X github.com/veritone/engine-tool/engine.BuildTag=$(githash)

.PHONY: clean test buildrelease release

release: test clean buildrelease

buildrelease:
	rm -rf dist
	mkdir dist
	GOOS=linux GOARCH=amd64 go build -ldflags "${ldflags}" -o release/bin/engine
	cp -a release dist/engine-toolkit-sdk-v0.1.7
	cd dist && tar cvvzf engine-toolkit-sdk-v0.1.7.tar.gz engine-toolkit-sdk-v0.1.7

clean:
	rm -rf dist
	rm -f release/bin/engine

test:
	go test -v ./...
