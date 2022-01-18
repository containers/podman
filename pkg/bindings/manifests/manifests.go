package manifests

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/images"
	"github.com/containers/podman/v4/version"
	jsoniter "github.com/json-iterator/go"
)

// Create creates a manifest for the given name.  Optional images to be associated with
// the new manifest can also be specified.  The all boolean specifies to add all entries
// of a list if the name provided is a manifest list.  The ID of the new manifest list
// is returned as a string.
func Create(ctx context.Context, name string, images []string, options *CreateOptions) (string, error) {
	var idr handlers.IDResponse
	if options == nil {
		options = new(CreateOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}
	if len(name) < 1 {
		return "", errors.New("creating a manifest requires at least one name argument")
	}
	params, err := options.ToParams()
	if err != nil {
		return "", err
	}

	for _, i := range images {
		params.Add("images", i)
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/manifests/%s", params, nil, name)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return idr.ID, response.Process(&idr)
}

// Exists returns true if a given manifest list exists
func Exists(ctx context.Context, name string, options *ExistsOptions) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/manifests/%s/exists", nil, nil, name)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	return response.IsSuccess(), nil
}

// Inspect returns a manifest list for a given name.
func Inspect(ctx context.Context, name string, _ *InspectOptions) (*manifest.Schema2List, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/manifests/%s/json", nil, nil, name)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var list manifest.Schema2List
	return &list, response.Process(&list)
}

// Add adds a manifest to a given manifest list.  Additional options for the manifest
// can also be specified.  The ID of the new manifest list is returned as a string
func Add(ctx context.Context, name string, options *AddOptions) (string, error) {
	if options == nil {
		options = new(AddOptions)
	}

	if bindings.ServiceVersion(ctx).GTE(semver.MustParse("4.0.0")) {
		optionsv4 := ModifyOptions{
			All:         options.All,
			Annotations: options.Annotation,
			Arch:        options.Arch,
			Features:    options.Features,
			Images:      options.Images,
			OS:          options.OS,
			OSFeatures:  nil,
			OSVersion:   options.OSVersion,
			Variant:     options.Variant,
		}
		optionsv4.WithOperation("update")
		return Modify(ctx, name, options.Images, &optionsv4)
	}

	// API Version < 4.0.0
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}
	opts, err := jsoniter.MarshalToString(options)
	if err != nil {
		return "", err
	}
	reader := strings.NewReader(opts)

	headers := make(http.Header)
	v := version.APIVersion[version.Libpod][version.MinimalAPI]
	headers.Add("API-Version",
		fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch))
	response, err := conn.DoRequest(ctx, reader, http.MethodPost, "/manifests/%s/add", nil, headers, name)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var idr handlers.IDResponse
	return idr.ID, response.Process(&idr)
}

// Remove deletes a manifest entry from a manifest list.  Both name and the digest to be
// removed are mandatory inputs.  The ID of the new manifest list is returned as a string.
func Remove(ctx context.Context, name, digest string, _ *RemoveOptions) (string, error) {
	if bindings.ServiceVersion(ctx).GTE(semver.MustParse("4.0.0")) {
		optionsv4 := new(ModifyOptions).WithOperation("remove")
		return Modify(ctx, name, []string{digest}, optionsv4)
	}

	// API Version < 4.0.0
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}

	headers := http.Header{}
	headers.Add("API-Version", "3.4.0")

	params := url.Values{}
	params.Set("digest", digest)
	response, err := conn.DoRequest(ctx, nil, http.MethodDelete, "/manifests/%s", params, headers, name)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var idr handlers.IDResponse
	return idr.ID, response.Process(&idr)
}

// Push takes a manifest list and pushes to a destination.  If the destination is not specified,
// the name will be used instead.  If the optional all boolean is specified, all images specified
// in the list will be pushed as well.
func Push(ctx context.Context, name, destination string, options *images.PushOptions) (string, error) {
	var (
		idr handlers.IDResponse
	)
	if options == nil {
		options = new(images.PushOptions)
	}
	if len(destination) < 1 {
		destination = name
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}

	params, err := options.ToParams()
	if err != nil {
		return "", err
	}
	// SkipTLSVerify is special.  We need to delete the param added by
	// ToParams() and change the key and flip the bool
	if options.SkipTLSVerify != nil {
		params.Del("SkipTLSVerify")
		params.Set("tlsVerify", strconv.FormatBool(!options.GetSkipTLSVerify()))
	}

	var response *bindings.APIResponse
	if bindings.ServiceVersion(ctx).GTE(semver.MustParse("4.0.0")) {
		response, err = conn.DoRequest(ctx, nil, http.MethodPost, "/manifests/%s/registry/%s", params, nil, name, destination)
	} else {
		params.Set("image", name)
		params.Set("destination", destination)
		response, err = conn.DoRequest(ctx, nil, http.MethodPost, "/manifests/%s/push", params, nil, name)
	}
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return idr.ID, err
}

// Modify modifies the given manifest list using options and the optional list of images
func Modify(ctx context.Context, name string, images []string, options *ModifyOptions) (string, error) {
	if options == nil || *options.Operation == "" {
		return "", errors.New(`the field ModifyOptions.Operation must be set to either "update" or "remove"`)
	}
	options.WithImages(images)

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}
	opts, err := jsoniter.MarshalToString(options)
	if err != nil {
		return "", err
	}
	reader := strings.NewReader(opts)

	response, err := conn.DoRequest(ctx, reader, http.MethodPut, "/manifests/%s", nil, nil, name)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var idr handlers.IDResponse
	return idr.ID, response.Process(&idr)
}

// Annotate modifies the given manifest list using options and the optional list of images
//
// As of 4.0.0
func Annotate(ctx context.Context, name string, images []string, options *ModifyOptions) (string, error) {
	options.WithOperation("annotate")
	return Modify(ctx, name, images, options)
}
