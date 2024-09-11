package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type HTTPMethod string

const (
	GET    HTTPMethod = "GET"
	POST   HTTPMethod = "POST"
	PUT    HTTPMethod = "PUT"
	DELETE HTTPMethod = "DELETE"
)

func JSONRequest(method HTTPMethod, url string, payload interface{}, headers map[string]string) ([]byte, error) {
	var body io.Reader

	if payload != nil {
		var buf *bytes.Buffer
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(data)

		body = buf
	} else {
		body = nil
	}

	req, err := http.NewRequest(string(method), url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return io.ReadAll(response.Body)
}
