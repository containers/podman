package handlers

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
)

// NewAPIDecoder returns a configured schema.Decoder
func NewAPIDecoder() *schema.Decoder {
	d := schema.NewDecoder()
	d.IgnoreUnknownKeys(true)
	d.RegisterConverter(map[string][]string{}, convertUrlValuesString)
	d.RegisterConverter(time.Time{}, convertTimeString)
	return d
}

// On client:
// 	v := map[string][]string{
//		"dangling": {"true"},
//	}
//
//	payload, err := jsoniter.MarshalToString(v)
//	if err != nil {
//		panic(err)
//	}
//	payload = url.QueryEscape(payload)
func convertUrlValuesString(query string) reflect.Value {
	f := map[string][]string{}

	err := json.Unmarshal([]byte(query), &f)
	if err != nil {
		logrus.Infof("convertUrlValuesString: Failed to Unmarshal %s: %s", query, err.Error())
	}

	return reflect.ValueOf(f)
}

func convertTimeString(query string) reflect.Value {
	t, err := time.Parse(time.RFC3339, query)
	if err != nil {
		logrus.Infof("convertTimeString: Failed to Unmarshal %s: %s", query, err.Error())

		return reflect.ValueOf(time.Time{})
	}
	return reflect.ValueOf(t)
}
