package handlers

import (
	"encoding/json"
	"reflect"
	"syscall"
	"time"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
)

// NewAPIDecoder returns a configured schema.Decoder
func NewAPIDecoder() *schema.Decoder {
	_ = ParseDateTime

	d := schema.NewDecoder()
	d.IgnoreUnknownKeys(true)
	d.RegisterConverter(map[string][]string{}, convertURLValuesString)
	d.RegisterConverter(time.Time{}, convertTimeString)
	d.RegisterConverter(define.ContainerStatus(0), convertContainerStatusString)

	var Signal syscall.Signal
	d.RegisterConverter(Signal, convertSignal)
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
func convertURLValuesString(query string) reflect.Value {
	f := map[string][]string{}

	err := json.Unmarshal([]byte(query), &f)
	if err != nil {
		logrus.Infof("convertURLValuesString: Failed to Unmarshal %s: %s", query, err.Error())
	}

	return reflect.ValueOf(f)
}

func convertContainerStatusString(query string) reflect.Value {
	result, err := define.StringToContainerStatus(query)
	if err != nil {
		logrus.Infof("convertContainerStatusString: Failed to parse %s: %s", query, err.Error())

		// We return nil here instead of result because reflect.ValueOf().IsValid() will be true
		// in github.com/gorilla/schema's decoder, which means there's no parsing error
		return reflect.ValueOf(nil)
	}

	return reflect.ValueOf(result)
}

// isZero() can be used to determine if parsing failed.
func convertTimeString(query string) reflect.Value {
	var (
		err error
		t   time.Time
	)

	for _, f := range []string{
		time.UnixDate,
		time.ANSIC,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RubyDate,
		time.Stamp,
		time.StampMicro,
		time.StampMilli,
		time.StampNano,
	} {
		t, err = time.Parse(f, query)
		if err == nil {
			return reflect.ValueOf(t)
		}

		if _, isParseError := err.(*time.ParseError); isParseError {
			// Try next format
			continue
		} else {
			break
		}
	}

	// We've exhausted all formats, or something bad happened
	if err != nil {
		logrus.Infof("convertTimeString: Failed to parse %s: %s", query, err.Error())
	}
	return reflect.ValueOf(time.Time{})
}

// ParseDateTime is a helper function to aid in parsing different Time/Date formats
// isZero() can be used to determine if parsing failed.
func ParseDateTime(query string) time.Time {
	return convertTimeString(query).Interface().(time.Time)
}

func convertSignal(query string) reflect.Value {
	signal, err := util.ParseSignal(query)
	if err != nil {
		logrus.Infof("convertSignal: Failed to parse %s: %s", query, err.Error())
	}
	return reflect.ValueOf(signal)
}
