package network

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
)

// Create makes a new CNI network configuration
func Create(ctx context.Context, options entities.NetworkCreateOptions, name *string) (*entities.NetworkCreateReport, error) {
	var report entities.NetworkCreateReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if name != nil {
		params.Set("name", *name)
	}
	networkConfig, err := jsoniter.MarshalToString(options)
	if err != nil {
		return nil, err
	}
	stringReader := strings.NewReader(networkConfig)
	response, err := conn.DoRequest(stringReader, http.MethodPost, "/networks/create", params, nil)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}

// Inspect returns low level information about a CNI network configuration
func Inspect(ctx context.Context, nameOrID string) ([]entities.NetworkInspectReport, error) {
	var reports []entities.NetworkInspectReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/networks/%s/json", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	return reports, response.Process(&reports)
}

// Remove deletes a defined CNI network configuration by name.  The optional force boolean
// will remove all containers associated with the network when set to true.  A slice
// of NetworkRemoveReports are returned.
func Remove(ctx context.Context, nameOrID string, force *bool) ([]*entities.NetworkRmReport, error) {
	var reports []*entities.NetworkRmReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if force != nil {
		params.Set("size", strconv.FormatBool(*force))
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/networks/%s", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	return reports, response.Process(&reports)
}

// List returns a summary of all CNI network configurations
func List(ctx context.Context) ([]*entities.NetworkListReport, error) {
	var (
		netList []*entities.NetworkListReport
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/networks/json", nil, nil)
	if err != nil {
		return netList, err
	}
	return netList, response.Process(&netList)
}
