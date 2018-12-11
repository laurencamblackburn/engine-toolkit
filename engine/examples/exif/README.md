# EXIF engine

This sample engine is a fully functional engine that runs on the Veritone platform.

It uses the `github.com/rwcarlsen/goexif` project to extract EXIF metadata from the chunks sent to it.

It is written in Go and makes use of the [Veritone Engine Toolkit](https://machinebox.io/veritone/engine-toolkit).

## Files

* `main.go` - Main engine code
* `main_test.go` - Test code
* `Dockerfile` - The description of the Docker container to build
* `manifest.json` - Veritone manifest file
* `Makefile` - Contains helpful scripts (see `make` commend)
* `testdata` - Folder containing files used in the unit tests
