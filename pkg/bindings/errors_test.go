package bindings

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/containers/podman/v5/pkg/domain/entities/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBindings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bindings Suite")
}

var _ = Describe("APIResponse Process method", func() {

	createMockResponse := func(jsonResponse string, statusCode int) *APIResponse {
		response := &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(bytes.NewBufferString(jsonResponse)),
			Header:     make(http.Header),
		}
		response.Header.Set("Content-Type", "application/json")
		return &APIResponse{Response: response}
	}

	Describe("when processing SystemPruneReport", func() {
		Context("with the exact JSON that caused the original marshalling error", func() {
			It("should successfully unmarshal the response", func() {
				// This is the exact JSON that was causing the unmarshalling error
				jsonResponse := `{
					"PodPruneReport": null,
					"ContainerPruneReports": [
						{
							"Id": "aec04392e9b2fe7c4a36bc0cfa206dee35d7e403f7189df658ce909ccd598db7",
							"Size": 8219
						},
						{
							"Id": "3d8a8789524a0c44a61baa49ceedda7be069b0b3d01255b24013d2fb82168c7e",
							"Err": "replacing mount point \"/tmp/CI_7Qsy/podman-e2e-213135586/subtest-1767990215/p/root/overlay/d9f554276b923c07bf708858b5f35774f9d2924fa4094b1583e56b33ae357af1/merged\": directory not empty",
							"Size": 7238
						},
						{
							"Id": "e9ef46f3a3cd43c929b19a01013be4d052bcb228333e61dcb8eb7dd270ae44c2",
							"Size": 0
						}
					],
					"ImagePruneReports": null,
					"NetworkPruneReports": null,
					"VolumePruneReports": null,
					"ReclaimedSpace": 15457
				}`

				apiResponse := createMockResponse(jsonResponse, 200)
				var report types.SystemPruneReport

				err := apiResponse.Process(&report)
				Expect(err).ToNot(HaveOccurred())

				Expect(report.ContainerPruneReports).To(HaveLen(3))
				Expect(report.ReclaimedSpace).To(Equal(uint64(15457)))

				first := report.ContainerPruneReports[0]
				Expect(first.Id).To(Equal("aec04392e9b2fe7c4a36bc0cfa206dee35d7e403f7189df658ce909ccd598db7"))
				Expect(first.Size).To(Equal(uint64(8219)))
				Expect(first.Err).ToNot(HaveOccurred())

				second := report.ContainerPruneReports[1]
				Expect(second.Id).To(Equal("3d8a8789524a0c44a61baa49ceedda7be069b0b3d01255b24013d2fb82168c7e"))
				Expect(second.Size).To(Equal(uint64(7238)))
				Expect(second.Err).To(HaveOccurred())
				expectedErr := `replacing mount point "/tmp/CI_7Qsy/podman-e2e-213135586/subtest-1767990215/p/root/overlay/d9f554276b923c07bf708858b5f35774f9d2924fa4094b1583e56b33ae357af1/merged": directory not empty`
				Expect(second.Err.Error()).To(Equal(expectedErr))

				third := report.ContainerPruneReports[2]
				Expect(third.Id).To(Equal("e9ef46f3a3cd43c929b19a01013be4d052bcb228333e61dcb8eb7dd270ae44c2"))
				Expect(third.Size).To(Equal(uint64(0)))
				Expect(third.Err).ToNot(HaveOccurred())
			})
		})
	})
})
