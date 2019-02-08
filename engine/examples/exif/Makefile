githash = $(shell git rev-parse --short HEAD)
ldflags = -X main.BuildTag=$(githash)

build: clean buildengine builddocker
run: down buildengine builddocker up
runtestmode: down buildengine builddocker uptestmode

buildengine:
	GOOS=linux GOARCH=amd64 go build -ldflags "${ldflags}" -o ./dist/engine ../../
	GOOS=linux GOARCH=amd64 go build -o ./dist/exif-extraction-engine

builddocker:
	docker build -f Dockerfile -t exif-extraction-engine .

up:
	docker run -t exif-extraction-engine --name exif-extraction-engine

down:
	docker rm -f exif-extraction-engine 2>/dev/null || true

uptestmode:
	docker run -e "VERITONE_TESTMODE=true" -p 9090:9090 -p 8080:8080 --name exif-extraction-engine -t exif-extraction-engine 

clean:
	rm -rf ./dist

push:
	docker tag exif-extraction-engine docker.veritone.com/15170/exif-extraction-engine:beta
	docker push docker.veritone.com/15170/exif-extraction-engine:beta
