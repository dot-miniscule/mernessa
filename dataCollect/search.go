/**
 * Provides advanced searching capabilities for querying StackExchange
 */

package dataCollect

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/laktek/Stack-on-Go/stackongo"
)

var host string = "https://api.stackexchange.com" // API host site

// Searches for questions on StackExchange API that match parameters
func searchAdvanced(params map[string]string, transport http.RoundTripper) (*stackongo.Questions, error) {
	request_path := "search/advanced"
	output := new(stackongo.Questions)
	if err := get(transport, request_path, params, output); err != nil {
		return nil, fmt.Errorf("dataCollect/search.go error: %v", err.Error())
	}
	return output, nil
}

// Sends a Get request through the transport and parses the response into collection
func get(transport http.RoundTripper, section string, params map[string]string, collection interface{}) error {
	client := &http.Client{Transport: transport}
	response, err := client.Get(setupEndpoint(section, params).String())
	if err != nil {
		return fmt.Errorf("dataCollect/search.go error: %v", err.Error())
	}

	err = parseResponse(response, collection)
	if err != nil {
		return fmt.Errorf("dataCollect/search.go error: %v", err.Error())
	}

	return nil
}

// Return URL with params joined to path
func setupEndpoint(path string, params map[string]string) *url.URL {
	base_url, _ := url.Parse(host)
	endpoint, _ := base_url.Parse("/2.2/" + path)

	query := endpoint.Query()
	for key, value := range params {
		query.Set(key, value)
	}

	endpoint.RawQuery = query.Encode()

	return endpoint
}

// Parse response into result
func parseResponse(response *http.Response, result interface{}) error {
	defer response.Body.Close()

	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("dataCollect/search.go error: %v", err.Error())
	}

	if err := json.Unmarshal(bytes, result); err != nil {
		return fmt.Errorf("dataCollect/search.go error: %v", err.Error())
	}

	if response.StatusCode == 400 {
		return fmt.Errorf("Bad request: %s", string(bytes))
	}
	return nil
}
