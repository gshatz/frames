/*
Copyright 2018 Iguazio Systems Ltd.

Licensed under the Apache License, Version 2.0 (the "License") with
an addition restriction as set forth herein. You may not use this
file except in compliance with the License. You may obtain a copy of
the License at http://www.apache.org/licenses/LICENSE-2.0.

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
implied. See the License for the specific language governing
permissions and limitations under the License.

In addition, you may not use the software for any purposes that are
illegal under applicable law, and the grant of the foregoing license
under the Apache 2.0 license is conditioned upon your compliance with
such restriction.
*/

package frames

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/nuclio/logger"
	"github.com/pkg/errors"
)

// Client is v3io streaming client
type Client struct {
	URL    string
	apiKey string
	logger logger.Logger
	err    error // last error
}

// NewClient returns a new client
func NewClient(url string, apiKey string, logger logger.Logger) (*Client, error) {
	var err error
	if logger == nil {
		logger, err = NewLogger("info")
		if err != nil {
			return nil, errors.Wrap(err, "Can't create logger")
		}
	}

	client := &Client{
		URL:    url,
		apiKey: apiKey,
		logger: logger,
	}

	return client, nil
}

// Read runs a query on the client
func (c *Client) Read(request *ReadRequest) (FrameIterator, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(request); err != nil {
		return nil, errors.Wrap(err, "can't encode query")
	}

	req, err := http.NewRequest("POST", c.URL, &buf)
	if err != nil {
		return nil, errors.Wrap(err, "can't create HTTP request")
	}

	req.Header.Set("Autohrization", c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "can't call API")
	}

	it := &streamFrameIterator{
		reader:  resp.Body,
		decoder: NewDecoder(resp.Body),
		logger:  c.logger,
	}

	return it, nil
}

// streamFrameIterator implements FrameIterator over io.Reader
type streamFrameIterator struct {
	frame   Frame
	err     error
	reader  io.Reader
	decoder *Decoder
	logger  logger.Logger
}

func (it *streamFrameIterator) Next() bool {
	var err error

	it.frame, err = it.decoder.Decode()
	if err == nil {
		return true
	}

	if err == io.EOF {
		closer, ok := it.reader.(io.Closer)
		if ok {
			if err := closer.Close(); err != nil {
				it.logger.WarnWith("can't close reader", "error", err)
			}
		}

		return false
	}

	it.err = err
	return false
}

func (it *streamFrameIterator) At() Frame {
	return it.frame
}

func (it *streamFrameIterator) Err() error {
	return it.err
}
