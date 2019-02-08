build: weights buildengine builddocker
run: down weights buildengine builddocker up
runtestmode: down weights buildengine builddocker uptestmode

builddocker:	
	docker build -f Dockerfile -t pyimagenet .

buildengine:
	GOOS=linux GOARCH=amd64 go build -ldflags "${ldflags}" -o ./dist/engine ../../

weights:
	test -s ./weights/resnet50_weights_tf_dim_ordering_tf_kernels.h5 || wget -P weights https://github.com/fchollet/deep-learning-models/releases/download/v0.2/resnet50_weights_tf_dim_ordering_tf_kernels.h5

up:
	docker run -p 8080:8080 -t pyimagenet --name pyimagenet 

down:
	docker rm -f pyimagenet 2>/dev/null || true

login:
	docker run -p 8080:8080 -i --entrypoint=/bin/bash -t pyimagenet

uptestmode:
	docker run -e "VERITONE_TESTMODE=true" -p 9090:9090 -p 8080:8080 --name pyimagenet -t pyimagenet 

push:
	docker tag pyimagenet docker.veritone.com/15170/pyimagenet:beta
	docker push docker.veritone.com/15170/pyimagenet:beta

.PHONY: build run builddocker up down clean push login  weights