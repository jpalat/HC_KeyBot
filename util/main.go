package util

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
)

// PrintDump prints dump of request, optionally writing it in the response
func PrintDump(w http.ResponseWriter, r *http.Request, write bool) {
	dump, _ := httputil.DumpRequest(r, true)
	log.Printf("%v", string(dump))
	if write == true {
		w.Write(dump)
	}
}

// Decode into a map[string]interface{} the JSON in the POST Request
func DecodePostJSON(r *http.Request, logging bool) (map[string]interface{}, error) {
	var err error
	var payLoad map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&payLoad)
	if logging == true {
		log.Printf("Parsed body:%v", payLoad)
	}
	return payLoad, err
}
