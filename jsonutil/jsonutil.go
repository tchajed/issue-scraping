// vim: ts=4:sw=4
package jsonutil

// Package to make working with decoding arbitrary JSON objects easier. Makes
// type assertions simpler and safer.

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// Fetch a JSON resource with an interface tuned for REST applications: the
// params are URL-encoded to be added to the baseURL and a decoded JSON value
// is returned.
func Get(baseURL string,
	params map[string]string) (v map[string]interface{}, err error) {
	p := url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}
	resp, err := http.Get(baseURL + "?" + p.Encode())
	if err != nil {
		return
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&v)
	return
}

// Safely convert an interface to a JSON object map.  If provided interface is
// nil, returns a new map (which can be safely indexed).
func GetMap(v interface{}) map[string]interface{} {
	if v == nil {
		return make(map[string]interface{})
	}
	return v.(map[string]interface{})
}

func GetInt(v interface{}) int {
	return int(v.(int64))
}

// Safely get a string value from a map
func GetString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return v.(string)
	}
	return ""
}
