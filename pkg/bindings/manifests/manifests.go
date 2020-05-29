package manifests

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings"
	jsoniter "github.com/json-iterator/go"
)

// Create creates a manifest for the given name.  Optional images to be associated with
// the new manifest can also be specified.  The all boolean specifies to add all entries
// of a list if the name provided is a manifest list.  The ID of the new manifest list
// is returned as a string.
func Create(ctx context.Context, names, images []string, all *bool) (string, error) {
	var idr handlers.IDResponse
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}
	if len(names) < 1 {
		return "", errors.New("creating a manifest requires at least one name argument")
	}
	params := url.Values{}
	if all != nil {
		params.Set("all", strconv.FormatBool(*all))
	}
	for _, name := range names {
		params.Add("name", name)
	}
	for _, i := range images {
		params.Add("image", i)
	}

	response, err := conn.DoRequest(nil, http.MethodPost, "/manifests/create", params, nil)
	if err != nil {
		return "", err
	}
	return idr.ID, response.Process(&idr)
}

// Inspect returns a manifest list for a given name.
func Inspect(ctx context.Context, name string) (*manifest.Schema2List, error) {
	var list manifest.Schema2List
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/manifests/%s/json", nil, nil, name)
	if err != nil {
		return nil, err
	}
	return &list, response.Process(&list)
}

// Add adds a manifest to a given manifest list.  Additional options for the manifest
// can also be specified.  The ID of the new manifest list is returned as a string
func Add(ctx context.Context, name string, options image.ManifestAddOpts) (string, error) {
	var idr handlers.IDResponse
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}
	optionsString, err := jsoniter.MarshalToString(options)
	if err != nil {
		return "", err
	}
	stringReader := strings.NewReader(optionsString)
	response, err := conn.DoRequest(stringReader, http.MethodPost, "/manifests/%s/add", nil, nil, name)
	if err != nil {
		return "", err
	}
	return idr.ID, response.Process(&idr)
}

// Remove deletes a manifest entry from a manifest list.  Both name and the digest to be
// removed are mandatory inputs.  The ID of the new manifest list is returned as a string.
func Remove(ctx context.Context, name, digest string) (string, error) {
	var idr handlers.IDResponse
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}
	params := url.Values{}
	params.Set("digest", digest)
	response, err := conn.DoRequest(nil, http.MethodDelete, "/manifests/%s", params, nil, name)
	if err != nil {
		return "", err
	}
	return idr.ID, response.Process(&idr)
}

// Push takes a manifest list and pushes to a destination.  If the destination is not specified,
// the name will be used instead.  If the optional all boolean is specified, all images specified
// in the list will be pushed as well.
func Push(ctx context.Context, name string, destination *string, all *bool) (string, error) {
	var (
		idr handlers.IDResponse
	)
	dest := name
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}
	params := url.Values{}
	params.Set("image", name)
	if destination != nil {
		dest = *destination
	}
	params.Set("destination", dest)
	if all != nil {
		params.Set("all", strconv.FormatBool(*all))
	}
	_, err = conn.DoRequest(nil, http.MethodPost, "/manifests/%s/push", params, nil, name)
	if err != nil {
		return "", err
	}
	return idr.ID, err
}

// There is NO annotate endpoint.  this binding could never work
// Annotate updates the image configuration of a given manifest list
//func Annotate(ctx context.Context, name, digest string, options image.ManifestAnnotateOpts) (string, error) {
//	var idr handlers.IDResponse
//	conn, err := bindings.GetClient(ctx)
//	if err != nil {
//		return "", err
//	}
//	params := url.Values{}
//	params.Set("digest", digest)
//	optionsString, err := jsoniter.MarshalToString(options)
//	if err != nil {
//		return "", err
//	}
//	stringReader := strings.NewReader(optionsString)
//	response, err := conn.DoRequest(stringReader, http.MethodPost, "/manifests/%s/annotate", params, name)
//	if err != nil {
//		return "", err
//	}
//	return idr.ID, response.Process(&idr)
//}
