# Python Keras/Tensorflow engine

This sample engine is a fully functional engine that runs on the Veritone platform.

It uses Python with the deep learing framework [Keras](https://keras.io) using Tensorflow and a backend.

The example uses a pre-trained model for ImageNet, to classify an image between 1000 different classes.

It is written in Python and makes use of the [Veritone Engine Toolkit](https://machinebox.io/veritone/engine-toolkit).

## Files

* `Dockerfile` - The description of the Docker container to build
* `manifest.json` - Veritone manifest file
* `Makefile` - Contains helpful scripts. 
* `testdata` - Folder containing files used in the unit tests
* `main.py` - Python Flask app with Keras and Tensorflow

## Get started

The `Dockerfile` contains the following line:

```docker
ADD ./dist/engine /app/engine
```

You can get the `engine` binary when you [Download the Veritone Engine Toolkit SDK](https://github.com/veritone/engine-toolkit/releases/latest). Place it inside this project in a folder called `dist`.

## Make the docker image

`$ make build`
