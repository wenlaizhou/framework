package framework

import (
	"net/http"
	"bytes"
	"io/ioutil"
	"encoding/json"
)

func Post(url string, headers map[string]string, data []byte) (int, map[string][]string, []byte, error) {
	return doRequest(POST, url, headers, "", data)
}

func PostJson(url string, headers map[string]string, data interface{}) (int, map[string][]string, []byte, error) {
	if data != nil {
		dataJson, _ := json.Marshal(data)
		return doRequest(POST, url, headers, ApplicationJson, dataJson)
	}
	return doRequest(POST, url, headers, ApplicationJson, nil)
}

func Get(url string, headers map[string]string) (int, map[string][]string, []byte, error) {

	return doRequest(GET, url, headers, "", nil)
}

func doRequest(method string, url string,
	headers map[string]string, contentType string,
	body []byte) (int, map[string][]string, []byte, error) {

	bodyReader := bytes.NewReader(body)

	if body != nil || len(body) <= 0 {
		bodyReader = nil
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if ProcessError(err) {
		return -1, nil, nil, err
	}

	client := &http.Client{}
	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	if len(contentType) > 0 {
		req.Header.Set(ContentType, contentType)
	}
	resp, err := client.Do(req)
	if ProcessError(err) {
		return -1, nil, nil, err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, respBody, nil
}
