# Introduction

Engines allow you to process files (or chunks of files like frames from a video) in the Veritone platform.

This toolkit is aimed at Cognition engines processing chunks. For example, images, frames from videos, video clips, and audio files. If you are building a different kind of engine, you may need to use lower level APIs.

Engines are deployed to the Veritone platform in Docker containers, which can be thought of as being like lightweight virtual machines. The platform will automatically spin up instances of your engine to meet demand.

To deploy an engine into production, you will need to perform the following tasks:

1. [Build your engine executable](#how-to-build-an-engine)
1. [Consume HTTP webhooks to provide integration and respond with appropriate JSON](#webhooks)
1. [Write tests for your code](#testing-your-webhooks)
1. [Download the Engine Toolkit SDK](#download-the-engine-toolkit-sdk)
1. [Write a `Dockerfile` which describes your engine](#writing-a-dockerfile)
1. [Deploy your engine to the Veritone platform](#deploy-to-veritone)

The rest of this guide describes how to do this, and provides additional reading for more advanced use cases.

# How to build an engine

An engine is an executable program, packaged along with its dependencies, into a [Docker container](#writing-a-dockerfile).
Engines listen on an HTTP address and implement [Webhooks](#webhooks), which are called when they receive work.

## Sample engines

To see the code for a complete working engine, choose from the list below:

* **Go** - [Golang EXIF engine project on GitHub](https://github.com/veritone/engine-toolkit/tree/master/engine/examples/exif)
* **Python** - [Python Keras/Tensorflow Imagenet image tagging engine project on GitHub](https://github.com/veritone/engine-toolkit/tree/master/engine/examples/python_imagenet)

> If you would like to contribute an additional example engine, please [open an issue to start a conversation](https://github.com/veritone/engine-toolkit/issues/new?title=sample+project).

## Webhooks

Engines must implement the following webhooks:

* [Ready webhook](#ready-webhook)
* [Process webhook](#process-webhook)

Each webhook provides unique functionality, and is therefore triggered by a unique HTTP request. The requests are described in the following sections.

> The webhook endpoints are configurable and are specified using environment variables in the [`Dockerfile`](#writing-a-dockerfile).

The webhooks are expected to return a `200 OK` successful response, otherwise the engine toolkit will retry the operation by making the same requests again.

### Ready webhook

The Ready webhook is used to determine if the engine is ready to start doing work or not.

```
GET /ready
```

The webhook should reply with a `503 Service Unavailable` status until the engine is ready to receive work, at which point it should reply to this webhook with a simple `200 OK` response.

### Process webhook

The Process webhook is used to perform some processing on a file (like the frame from a video).

```
POST /process
```

The body of the request is a multipart form (Where `Content-Type` is `multipart/form-data`) containing everything the engine needs to do its work.

The following fields will be posted to the Process webhook:

* `chunk` - (File) The file to process
* `chunkMimeType` - (string) The MIME type of the chunk (for example, `image/jpg`)
* `startOffsetMS` - (int) The start time of the chunk (for example, the timestamp of when a frame was extracted from a video)
* `endOffsetMS` - (int) The end time of the chunk (see `startOffsetMS`)
* `width` - (int) The width of the chunk
* `height` - (int) The height of the chunk

The following advanced fields are also included:

* `libraryId` - (string) ID of the library related to this task
* `libraryEngineModelId` - (string) ID of the library engine model related to this task
* `cacheURI` - (string) URL of the chunk source file
* `veritoneApiBaseUrl` - (string) The root URL for Veritone platform API requests
* `token` - (string) The token to use when making low level API requests
* `payload` - (string) JSON string containing the entire task payload

> As the Engine Toolkit evolves, we expect to add more fields here. If you notice something missing, please [open an issue and let us know](https://github.com/veritone/engine-toolkit/issues/new?title=request+fields).

#### Process webhook response

The handler for the Process webhook should return the results by writing a JSON response.

> Most languages and frameworks have very easy ways of consuming HTTP endpoints and writing JSON responses. It is recommended that you use existing libraries where possible.

##### Example webhook response: Faces

The following JSON is an example showing some faces that were found in the image.

```json
{
	"series": [{
		"startTimeMs": 1000,
		"endTimeMs": 2000,
		"object": {
			"type": "face",
			"confidence": 0.95,
			"boundingPoly": [
				{"x":0.3,"y":0.1}, {"x":0.5,"y":0.1},
				{"x":0.5,"y":0.9}, {"x":0.3,"y":0.9}
			]
		}
	}, {
		"startTimeMs": 5000,
		"endTimeMs": 6000,
		"object": {
			"type": "face",
			"confidence": 0.95,
			"boundingPoly": [
				{"x":0,"y":0}, {"x":1,"y":0},
				{"x":1,"y":1}, {"x":0,"y":1}
			]
		}
	}]
}
```

* `series` - (array) List of items found
* `series[].startTimeMs` - (int) The start time of the chunk
* `series[].endTimeMs` - (int) The end time of the chunk
* `series[].object` - (object) An object describing what was found
* `series[].object.type` - (string) The type of the object
* `series[].object.confidence` - (number) A number `0-1` of how confident the engine is about this object
* `series[].object.boundingPoly` - (array) Array of points that describe the [region within a larger image](#regions)

> If you have a question about what your engine should output and this documentation doesn't cover it, please [open an issue to start a conversation](https://github.com/veritone/engine-toolkit/issues/new).

#### Ignoring chunks

If your engine is not going to process a chunk, the Process webhook should return a `204 No Content` response.

The Engine Toolkit will report the chunk as ignored.

## Testing your webhooks

Since the [Webhooks](#webhooks) are just HTTP endpoints, you can test them by making your own HTTP requests directly.

## Download the Engine Toolkit SDK

To get started, you need to download the Engine Toolkit SDK. It contains tools that will be bundled into the Docker container when you deploy your engine to the Veritone platform.

* [Download the Engine Toolkit SDK from our GitHub project](https://github.com/veritone/engine-toolkit/releases/latest)

## Writing a `Dockerfile`

Veritone engines are Docker containers that run in the platform. To provide an engine, you must 
build a Docker container (or image) that can encapsulate your dependencies and execute your code.

A `Dockerfile` explains the steps that Docker needs to take in order to build a container.

The following is an example of an engine `Dockerfile`:

```docker
FROM alpine:latest
RUN apk --no-cache add ca-certificates

ADD ./your-engine /app/your-engine

# Add and configure the engine
ADD manifest.json /var/manifest.json
ADD ./dist/engine /app/engine
ENV VERITONE_WEBHOOK_READY="http://0.0.0.0:8888/readyz"
ENV VERITONE_WEBHOOK_PROCESS="http://0.0.0.0:8888/process"
ENTRYPOINT [ "/app/engine", "/app/your-engine" ]
```

The most common commands in a `Dockerfile` are:

* `FROM` describes the Docker container you are starting with
* `RUN` runs a command inside the container (In this case, `apk update && apk --no-cache add ca-certificates` ensures root certificates are installed so that the engine can access secure SSL files. You may or may not need this depending on which base you choose.)
* `ADD` adds files to the container during the build process
* `ENV` sets environment variables
* `ENTRYPOINT` describes the executable that is run when an instance of the container is created by the platform

> To learn more about Docker and what it can do, we recommend that you browse the official [Docker Documentation](https://docs.docker.com/).

> If you want to dig deeper into what can be done in your `Dockerfile`, you can [read the Dockerfile reference manual](https://docs.docker.com/engine/reference/builder/#from) or continue reading for a light overview.

### Understand the `Dockerfile`

The `Dockerfile` above describes a simple but complete engine. This section explains the components that make up an engine.

#### Manifest file

```docker
ADD manifest.json /var/manifest.json
```

The `manifest.json` file is a JSON configuration file the describes details about your engine, such as its `engineId`, which file-types it supports, what kind of data it outputs, and more.

> The platform will [automatically generate your manifest file](#automatically-generate-your-manifest-file).

Usually your `manifest.json` file sits alongside your `Dockerfile` in your engine project folder.

#### The `engine` executable

```docker
ADD ./dist/engine /app/engine
```

The `engine` executable gets added to your docker image as the entry point, and it acts as a proxy between the Veritone platform and your webhooks.

Specifically, it:

* Starts your engine process proxying the stdout and stderr
* Connects to the Veritone platform and receives work
* Calls your webhooks
* Writes your responses back to the platform

It is available when you [download the Engine Toolkit SDK](#download-the-engine-toolkit-sdk).

#### Webhook environment variables

```docker
ENV VERITONE_WEBHOOK_READY="http://0.0.0.0:8888/readyz"
ENV VERITONE_WEBHOOK_PROCESS="http://0.0.0.0:8888/process"
```

The environment variables specified with the `ENV` command inform the `engine` tool which webhooks to hit.

* `VERITONE_WEBHOOK_READY` - (string) Complete URL (usually local) of your Ready webhook
* `VERITONE_WEBHOOK_PROCESS` - (string) Complete URL (usually local) of your Process webhook

#### Engine entrypoint

```docker
ENTRYPOINT [ "/app/engine", "/app/your-engine", "--your-arg-1", "--your-arg-2" ]
```

The `ENTRYPOINT` must always be the [`engine` executable](#the-engine-executable) as the first argument, but the other arguments are for you to use to specify your engine code process.

* `/app/engine` - (required constant string) Must be `/app/engine` since it refers to the [`engine` executable](#the-engine-executable)
* `/app/your-engine` - (required string) The path and name of your custom engine executable
* `--your-arg-n` - (optional strings) Additional arguments to pass when running your custom engine executable

# Deploy to Veritone

To deploy your code in Veritone, you must first create an engine on the platform.

> This process is trivial and easy to clean up, so don't worry about getting the forms exactly right the very first time. This guide will call out any important fields.

1. Go to [https://developer.veritone.com/](https://developer.veritone.com/) and sign in
1. Click on <strong>ENGINES</strong> in the navigation
1. Click <strong>NEW</strong> and select <strong>ENGINE</strong> from the menu to create a new engine
1. Select <strong>Cognition</strong> engine type
1. Complete the form selecting the appropriate or closest match options from the <strong>Engine Category</strong>
1. Only check <strong>Library Required</strong> if your engine code knows how to interact with the Veritone Library via the GraphQL API
1. Click <strong>Next</strong>
1. Select your deployment model that describes your engine's needs
1. Click <strong>Next</strong>&mdash;you may skip the <strong>Custom Fields</strong> page if this is the first time creating an engine
1. Click <strong>Submit</strong>

Your engine will be created and you'll be redirected to its console page.

## Automatically generate your manifest file

In the in-app documentation of your [engine's console page](https://developer.veritone.com), click <strong>GENERATE MANIFEST FILE</strong> to have the platform generate your [manifest.json file](#manifest-file).

The rest of the in-app documentation explains how to authenticate the Docker tools and use them to tag and push your engine.

# Development guides

This section includes some helpful guides for how to solve common problems when building engines.

## Regions

To specify a region (such as an area on an image) the Veritone platform expects a relative array of `x,y` points where each value is between `0` and `1`, with `0` being the left or top of an image and `1` being the right or bottom.

Relative (or ratio) values are used so that they remain correct regardless of the resolution of the image.

```json
"boundingPoly": [
	{"x": 0, "y": 0},
	{"x": 1, "y": 0},
	{"x": 1, "y": 1},
	{"x": 0, "y": 1}
]
```

> This example essentially draws a box around the entire image.

The following diagram represents the points for a box describing an object that is `50x50` in the center of a `100x100` image:

![](/veritone/engine-toolkit/static/boundingpoly.png)

The `boundingPoly` array for this object would be:

```json
"boundingPoly": [
	{"x": 0.25, "y": 0.25},
	{"x": 0.75, "y": 0.25},
	{"x": 0.75, "y": 0.75},
	{"x": 0.25, "y": 0.75}
]
```

### Calculating the ratio value

To calculate the `x` and `y` ratio values, you divide `x` by the width and `y` by the height:

```
ratioX = x / width
ratioY = y / height
```
