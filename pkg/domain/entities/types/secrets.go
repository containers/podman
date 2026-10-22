package types

import (
	"encoding/json"
	"errors"
	"time"
)

type SecretSpec struct {
	Name   string
	Driver SecretDriverSpec
	Labels map[string]string
}

type SecretVersion struct {
	Index int
}

type SecretDriverSpec struct {
	Name    string
	Options map[string]string
}

type SecretCreateReport struct {
	ID string
}

type SecretListReport struct {
	ID        string
	Name      string
	Driver    string
	CreatedAt string
	UpdatedAt string
}

type SecretRmReport struct {
	ID  string
	Err error
}

// secretRmReportJSON is the wire form. The service encodes Err with
// json-iterator, which writes an error as a string. encoding/json cannot
// store that string in an error field.
type secretRmReportJSON struct {
	ID  string
	Err string
}

func (r SecretRmReport) MarshalJSON() ([]byte, error) {
	helper := secretRmReportJSON{ID: r.ID}
	if r.Err != nil {
		helper.Err = r.Err.Error()
	}
	return json.Marshal(helper)
}

func (r *SecretRmReport) UnmarshalJSON(data []byte) error {
	var helper secretRmReportJSON
	if err := json.Unmarshal(data, &helper); err != nil {
		return err
	}
	r.ID = helper.ID
	if helper.Err == "" {
		r.Err = nil
		return nil
	}
	r.Err = errors.New(helper.Err)
	return nil
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
