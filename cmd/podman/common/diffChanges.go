package common

import (
	"fmt"
	"os"

	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
)

type ChangesReportJSON struct {
	Changed []string `json:"changed,omitempty"`
	Added   []string `json:"added,omitempty"`
	Deleted []string `json:"deleted,omitempty"`
}

func ChangesToJSON(diffs *entities.DiffReport) error {
	body := ChangesReportJSON{}
	for _, row := range diffs.Changes {
		switch row.Kind {
		case archive.ChangeAdd:
			body.Added = append(body.Added, row.Path)
		case archive.ChangeDelete:
			body.Deleted = append(body.Deleted, row.Path)
		case archive.ChangeModify:
			body.Changed = append(body.Changed, row.Path)
		default:
			return errors.Errorf("output kind %q not recognized", row.Kind)
		}
	}

	// Pull in configured json library
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(body)
}

func ChangesToTable(diffs *entities.DiffReport) error {
	for _, row := range diffs.Changes {
		fmt.Fprintln(os.Stdout, row.String())
	}
	return nil
}
