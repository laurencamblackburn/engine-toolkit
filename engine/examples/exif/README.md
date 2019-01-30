# EXIF engine

This sample engine is a fully functional engine that runs on the Veritone platform.

It uses the `github.com/rwcarlsen/goexif` project to extract EXIF metadata from the chunks sent to it.

It is written in Go and makes use of the [Veritone Engine Toolkit](https://machinebox.io/veritone/engine-toolkit).

## Get started

The `Dockerfile` contains the following line:

```docker
ADD ./dist/engine /app/engine
```

You can get the `engine` binary when you [Download the Veritone Engine Toolkit SDK](https://github.com/veritone/engine-toolkit/releases/latest). Place it inside this project in a folder called `dist`.

## Files

* `main.go` - Main engine code
* `main_test.go` - Test code
* `Dockerfile` - The description of the Docker container to build
* `manifest.json` - Veritone manifest file
* `Makefile` - Contains helpful scripts (see `make` commend)
* `testdata` - Folder containing files used in the unit tests
