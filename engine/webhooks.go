package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
)

func newRequestFromMediaChunk(client *http.Client, processURL string, msg mediaChunkMessage) (*http.Request, error, TaskFailureReason) {
	payload, err := msg.unmarshalPayload()
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal payload"), FailureReasonInvalidData
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("chunkMimeType", msg.MIMEType)
	w.WriteField("chunkIndex", strconv.Itoa(msg.ChunkIndex))
	w.WriteField("startOffsetMS", strconv.Itoa(msg.StartOffsetMS))
	w.WriteField("endOffsetMS", strconv.Itoa(msg.EndOffsetMS))
	w.WriteField("width", strconv.Itoa(msg.Width))
	w.WriteField("height", strconv.Itoa(msg.Height))
	w.WriteField("libraryId", payload.LibraryID)
	w.WriteField("libraryEngineModelId", payload.LibraryEngineModelID)
	w.WriteField("cacheURI", msg.CacheURI)
	w.WriteField("veritoneApiBaseUrl", payload.VeritoneAPIBaseURL)
	w.WriteField("token", payload.Token)
	w.WriteField("payload", string(msg.TaskPayload))
	if msg.CacheURI != "" {
		f, err := w.CreateFormFile("chunk", "chunk.data")
		if err != nil {
			return nil, err, FailureReasonFileWriteError
		}
		resp, err := client.Get(msg.CacheURI)
		if err != nil {
			return nil, errors.Wrap(err, "download chunk file from source"), FailureReasonURLNotFound
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, errors.Wrapf(err, "download chunk file from source: status code %v", resp.Status), FailureReasonURLNotAllowed
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			return nil, errors.Wrap(err, "read chunk file from source"), FailureReasonURLNotAllowed
		}
	}
	if err := w.Close(); err != nil {
		return nil, err, FailureReasonSystemDependencyMissing
	}
	req, err := http.NewRequest(http.MethodPost, processURL, &buf)
	if err != nil {
		return nil, err, FailureReasonOther
	}
	req.Header.Set("User-Agent", "veritone-engine-toolkit")
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil, ""
}
