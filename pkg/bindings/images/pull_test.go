package images

import (
	"bytes"
	"strings"
	"testing"

	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/docker/docker/pkg/jsonmessage"
)

func Test_printStreamLine(t *testing.T) {
	tests := []struct {
		name     string
		report   *entities.ImagePullReport
		contains string
		wantErr  bool
	}{
		{
			name: "Downloading blob with progress",
			report: &entities.ImagePullReport{
				Stream: "Copying blob",
				ID:     "b9ed43dcc389",
				Progress: &jsonmessage.JSONProgress{
					Total:   1024,
					Current: 512,
				},
			},
			contains: "Copying blob b9ed43dcc389 [=========================>                         ]     512B/1.024kB",
		},
		{
			name: "Blob download complete",
			report: &entities.ImagePullReport{
				Stream: "Copying blob",
				ID:     "b9ed43dcc389",
				Status: "done",
			},
			contains: "Copying blob b9ed43dcc389 done",
		},
		{
			name: "Only stream message",
			report: &entities.ImagePullReport{
				Stream: "Stream message",
			},
			contains: "Stream message",
		},
		{
			name: "Report contains error",
			report: &entities.ImagePullReport{
				Stream: "Any message",
				Error:  "Error",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			if err := printStreamLine(tt.report, out); (err != nil) != tt.wantErr {
				t.Errorf("printStreamLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// compare as substring due to a special line manipulating chars
			if gotOut := out.String(); !strings.Contains(gotOut, tt.contains) {
				t.Errorf("printStreamLine() = %s, to contain %s", gotOut, tt.contains)
			}
		})
	}
}
