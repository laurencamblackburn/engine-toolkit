# Engine Toolkit Minisite

This is the code for the website running at https://machinebox.io/veritone/engine-toolkit.

## Dev

To run:

```
PORT=8080 go run *.go
```

or
```
make fresh
```

To deploy to Google App Engine:

```
gcloud app deploy app.yaml -v v1
```

* The `gcloud` command comes from the [Google Cloud SDK](https://cloud.google.com/sdk/)
