package framework

import (
	"net/http"
	"bytes"
	"io/ioutil"
	"encoding/json"
)

//Post: post data to url
//
//return : statusCode, header, body, error
func Post(url string, headers map[string]string, data []byte) (int, map[string][]string, []byte, error) {
	return DoRequest(POST, url, headers, "", data)
}

//PostJson: post json data to url, contentType设置为: application/json utf8
//
//data : interface, 自动将其转成json格式
//
//return : statusCode, header, body, error
func PostJson(url string, headers map[string]string, data interface{}) (int, map[string][]string, []byte, error) {
	if data != nil {
		dataJson, _ := json.Marshal(data)
		return DoRequest(POST, url, headers, ApplicationJson, dataJson)
	}
	return DoRequest(POST, url, headers, ApplicationJson, nil)
}

//Get: get data from url
//
//return : statusCode, header, body, error
func Get(url string, headers map[string]string) (int, map[string][]string, []byte, error) {

	return DoRequest(GET, url, headers, "", nil)
}

//DoRequest: post data to url
//
//return : statusCode, header, body, error
func DoRequest(method string, url string,
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
