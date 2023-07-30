package entities

import (
	"time"

	"github.com/containers/podman/v4/pkg/errorhandling"
)

type SecretCreateReport struct {
	ID string
}

type SecretCreateOptions struct {
	Driver     string
	DriverOpts map[string]string
	Labels     map[string]string
	Replace    bool
}

type SecretInspectOptions struct {
	ShowSecret bool
}

type SecretListRequest struct {
	Filters map[string][]string
}

type SecretListReport struct {
	ID        string
	Name      string
	Driver    string
	CreatedAt string
	UpdatedAt string
}

type SecretRmOptions struct {
	All    bool
	Ignore bool
}

type SecretRmReport struct {
	ID  string
	Err error
}

type SecretInfoReport struct {
	ID         string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Spec       SecretSpec
	SecretData string `json:"SecretData,omitempty"`
}

type SecretInfoReportCompat struct {
	SecretInfoReport
	Version SecretVersion
}

type SecretVersion struct {
	Index int
}

type SecretSpec struct {
	Name   string
	Driver SecretDriverSpec
	Labels map[string]string
}

type SecretDriverSpec struct {
	Name    string
	Options map[string]string
}

// swagger:model SecretCreate
type SecretCreateRequest struct {
	// User-defined name of the secret.
	Name string
	// Base64-url-safe-encoded (RFC 4648) data to store as secret.
	Data string
	// Driver represents a driver (default "file")
	Driver SecretDriverSpec
	// Labels are labels on the secret
	Labels map[string]string
}

// Secret create response
// swagger:response SecretCreateResponse
type SwagSecretCreateResponse struct {
	// in:body
	Body struct {
		SecretCreateReport
	}
}

// Secret list response
// swagger:response SecretListResponse
type SwagSecretListResponse struct {
	// in:body
	Body []*SecretInfoReport
}

// Secret list response
// swagger:response SecretListCompatResponse
type SwagSecretListCompatResponse struct {
	// in:body
	Body []*SecretInfoReportCompat
}

// Secret inspect response
// swagger:response SecretInspectResponse
type SwagSecretInspectResponse struct {
	// in:body
	Body SecretInfoReport
}

// Secret inspect compat
// swagger:response SecretInspectCompatResponse
type SwagSecretInspectCompatResponse struct {
	// in:body
	Body SecretInfoReportCompat
}

// No such secret
// swagger:response NoSuchSecret
type SwagErrNoSuchSecret struct {
	// in:body
	Body struct {
		errorhandling.ErrorModel
	}
}

// Secret in use
// swagger:response SecretInUse
type SwagErrSecretInUse struct {
	// in:body
	Body struct {
		errorhandling.ErrorModel
	}
}
