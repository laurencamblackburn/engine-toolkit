build: weights builddocker
run: weights builddocker up

builddocker:	
	docker build -f Dockerfile -t pyimagenet .

weights:
	test -s ./weights/resnet50_weights_tf_dim_ordering_tf_kernels.h5 || wget -P weights https://github.com/fchollet/deep-learning-models/releases/download/v0.2/resnet50_weights_tf_dim_ordering_tf_kernels.h5

up:
	docker run -p 8080:8080 -t pyimagenet --name pyimagenet 

down:
	docker rm -f pyimagenet 2>/dev/null || true

login:
	docker run -p 8080:8080 -i --entrypoint=/bin/bash -t pyimagenet

push:
	docker tag pyimagenet docker.veritone.com/15170/pyimagenet:beta
	docker push docker.veritone.com/15170/pyimagenet:beta

.PHONY: build run builddocker up down clean push login  weights