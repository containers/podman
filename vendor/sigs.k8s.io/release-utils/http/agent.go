/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package http

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultPostContentType = "application/octet-stream"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//go:generate /usr/bin/env bash -c "cat ../scripts/boilerplate/boilerplate.generatego.txt httpfakes/fake_agent_implementation.go > httpfakes/_fake_agent_implementation.go && mv httpfakes/_fake_agent_implementation.go httpfakes/fake_agent_implementation.go"

// Agent is an http agent
type Agent struct {
	options *agentOptions
	AgentImplementation
}

// AgentImplementation is the actual implementation of the http calls
//counterfeiter:generate . AgentImplementation
type AgentImplementation interface {
	SendPostRequest(*http.Client, string, []byte, string) (*http.Response, error)
	SendGetRequest(*http.Client, string) (*http.Response, error)
}

type defaultAgentImplementation struct{}

// agentOptions has the configurable bits of the agent
type agentOptions struct {
	FailOnHTTPError bool          // Set to true to fail on HTTP Status > 299
	Retries         uint          // Number of times to retry when errors happen
	Timeout         time.Duration // Timeout when fetching URLs
	MaxWaitTime     time.Duration // Max waiting time when backing off
	PostContentType string        // Content type to send when posting data
}

// String returns a string representation of the options
func (ao *agentOptions) String() string {
	return fmt.Sprintf(
		"HTTP.Agent options: Timeout: %d - Retries: %d - FailOnHTTPError: %+v",
		ao.Timeout, ao.Retries, ao.FailOnHTTPError,
	)
}

var defaultAgentOptions = &agentOptions{
	FailOnHTTPError: true,
	Retries:         3,
	Timeout:         3 * time.Second,
	MaxWaitTime:     60 * time.Second,
	PostContentType: defaultPostContentType,
}

// NewAgent return a new agent with default options
func NewAgent() *Agent {
	return &Agent{
		AgentImplementation: &defaultAgentImplementation{},
		options:             defaultAgentOptions,
	}
}

// SetImplementation sets the agent implementation
func (a *Agent) SetImplementation(impl AgentImplementation) {
	a.AgentImplementation = impl
}

// WithTimeout sets the agent timeout
func (a *Agent) WithTimeout(timeout time.Duration) *Agent {
	a.options.Timeout = timeout
	return a
}

// WithRetries sets the number of times we'll attempt to fetch the URL
func (a *Agent) WithRetries(retries uint) *Agent {
	a.options.Retries = retries
	return a
}

// WithFailOnHTTPError determines if the agent fails on HTTP errors (HTTP status not in 200s)
func (a *Agent) WithFailOnHTTPError(flag bool) *Agent {
	a.options.FailOnHTTPError = flag
	return a
}

// Client return an net/http client preconfigured with the agent options
func (a *Agent) Client() *http.Client {
	return &http.Client{
		Timeout: a.options.Timeout,
	}
}

// Get resturns the body a a GET request
func (a *Agent) Get(url string) (content []byte, err error) {
	request, err := a.GetRequest(url)
	if err != nil {
		return nil, errors.Wrap(err, "getting GET request")
	}
	defer request.Body.Close()

	return a.readResponse(request)
}

// GetRequest sends a GET request to a URL and returns the request and response
func (a *Agent) GetRequest(url string) (response *http.Response, err error) {
	logrus.Infof("Sending GET request to %s", url)
	logrus.Debug()
	try := 0
	for {
		response, err = a.AgentImplementation.SendGetRequest(a.Client(), url)
		try++
		if err == nil || try >= int(a.options.Retries) {
			return response, err
		}
		// Do exponential backoff...
		waitTime := math.Pow(2, float64(try))
		//  ... but wait no more than 1 min
		if waitTime > 60 {
			waitTime = a.options.MaxWaitTime.Seconds()
		}
		logrus.Errorf(
			"Error getting URL (will retry %d more times in %.0f secs): %s",
			int(a.options.Retries)-try, waitTime, err.Error(),
		)
		time.Sleep(time.Duration(waitTime) * time.Second)
	}
}

// Post returns the body of a POST request
func (a *Agent) Post(url string, postData []byte) (content []byte, err error) {
	response, err := a.PostRequest(url, postData)
	if err != nil {
		return nil, errors.Wrap(err, "getting post request")
	}
	defer response.Body.Close()

	return a.readResponse(response)
}

// PostRequest sends the postData in a POST request to a URL and returns the request object
func (a *Agent) PostRequest(url string, postData []byte) (response *http.Response, err error) {
	logrus.Infof("Sending POST request to %s", url)
	logrus.Debug(a.options.String())

	try := 0
	for {
		response, err = a.AgentImplementation.SendPostRequest(a.Client(), url, postData, a.options.PostContentType)
		try++
		if err == nil || try >= int(a.options.Retries) {
			return response, err
		}
		// Do exponential backoff...
		waitTime := math.Pow(2, float64(try))
		//  ... but wait no more than 1 min
		if waitTime > 60 {
			waitTime = a.options.MaxWaitTime.Seconds()
		}
		logrus.Errorf(
			"Error getting URL (will retry %d more times in %.0f secs): %s",
			int(a.options.Retries)-try, waitTime, err.Error(),
		)
		time.Sleep(time.Duration(waitTime) * time.Second)
	}
}

// SendPostRequest sends the actual HTTP post to the server
func (impl *defaultAgentImplementation) SendPostRequest(
	client *http.Client, url string, postData []byte, contentType string,
) (response *http.Response, err error) {
	if contentType == "" {
		contentType = defaultPostContentType
	}
	response, err = client.Post(url, contentType, bytes.NewBuffer(postData))
	if err != nil {
		return response, errors.Wrapf(err, "posting data to %s", url)
	}
	return response, nil
}

// SendGetRequest performs the actual request
func (impl *defaultAgentImplementation) SendGetRequest(client *http.Client, url string) (
	response *http.Response, err error,
) {
	response, err = client.Get(url)
	if err != nil {
		return response, errors.Wrapf(err, "getting %s", url)
	}

	return response, nil
}

// readResponse read an dinterpret the http request
func (a *Agent) readResponse(response *http.Response) (body []byte, err error) {
	// Read the response body
	defer response.Body.Close()
	body, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrapf(
			err, "reading the response body from %s", response.Request.URL)
	}

	// Check the https response code
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if a.options.FailOnHTTPError {
			return nil, errors.New(fmt.Sprintf(
				"HTTP error %s for %s", response.Status, response.Request.URL,
			))
		}
		logrus.Warnf("Got HTTP error but FailOnHTTPError not set: %s", response.Status)
	}
	return body, err
}
