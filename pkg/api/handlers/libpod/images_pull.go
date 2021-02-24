package libpod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/image"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/channel"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ImagesPull is the v2 libpod endpoint for pulling images.  Note that the
// mandatory `reference` must be a reference to a registry (i.e., of docker
// transport or be normalized to one).  Other transports are rejected as they
// do not make sense in a remote context.
func ImagesPull(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Reference string `schema:"reference"`
		OS        string `schema:"OS"`
		Arch      string `schema:"Arch"`
		Variant   string `schema:"Variant"`
		TLSVerify bool   `schema:"tlsVerify"`
		AllTags   bool   `schema:"allTags"`
	}{
		TLSVerify: true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	if len(query.Reference) == 0 {
		utils.InternalServerError(w, errors.New("reference parameter cannot be empty"))
		return
	}

	imageRef, err := utils.ParseDockerReference(query.Reference)
	if err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, err)
		return
	}

	// Trim the docker-transport prefix.
	rawImage := strings.TrimPrefix(query.Reference, fmt.Sprintf("%s://", docker.Transport.Name()))

	// all-tags doesn't work with a tagged reference, so let's check early
	namedRef, err := reference.Parse(rawImage)
	if err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "error parsing reference %q", rawImage))
		return
	}
	if _, isTagged := namedRef.(reference.Tagged); isTagged && query.AllTags {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Errorf("reference %q must not have a tag for all-tags", rawImage))
		return
	}

	authConf, authfile, key, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "failed to retrieve repository credentials", http.StatusBadRequest, errors.Wrapf(err, "failed to parse %q header for %s", key, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)

	// Setup the registry options
	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds: authConf,
		OSChoice:            query.OS,
		ArchitectureChoice:  query.Arch,
		VariantChoice:       query.Variant,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	sys := runtime.SystemContext()
	if sys == nil {
		sys = image.GetSystemContext("", authfile, false)
	}
	dockerRegistryOptions.DockerCertPath = sys.DockerCertPath
	sys.DockerAuthConfig = authConf

	// Prepare the images we want to pull
	imagesToPull := []string{}
	imageName := namedRef.String()

	if !query.AllTags {
		imagesToPull = append(imagesToPull, imageName)
	} else {
		tags, err := docker.GetRepositoryTags(context.Background(), sys, imageRef)
		if err != nil {
			utils.InternalServerError(w, errors.Wrap(err, "error getting repository tags"))
			return
		}
		for _, tag := range tags {
			imagesToPull = append(imagesToPull, fmt.Sprintf("%s:%s", imageName, tag))
		}
	}

	writer := channel.NewWriter(make(chan []byte))
	defer writer.Close()

	stderr := channel.NewWriter(make(chan []byte))
	defer stderr.Close()

	images := make([]string, 0, len(imagesToPull))
	runCtx, cancel := context.WithCancel(context.Background())
	go func(imgs []string) {
		defer cancel()
		// Finally pull the images
		for _, img := range imgs {
			newImage, err := runtime.ImageRuntime().New(
				runCtx,
				img,
				"",
				authfile,
				writer,
				&dockerRegistryOptions,
				image.SigningOptions{},
				nil,
				util.PullImageAlways,
				nil)
			if err != nil {
				stderr.Write([]byte(err.Error() + "\n"))
			} else {
				images = append(images, newImage.ID())
			}
		}
	}(imagesToPull)

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Add("Content-Type", "application/json")
	flush()

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	var failed bool
loop: // break out of for/select infinite loop
	for {
		var report entities.ImagePullReport
		select {
		case e := <-writer.Chan():
			report.Stream = string(e)
			if err := enc.Encode(report); err != nil {
				stderr.Write([]byte(err.Error()))
			}
			flush()
		case e := <-stderr.Chan():
			failed = true
			report.Error = string(e)
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
		case <-runCtx.Done():
			if !failed {
				// Send all image id's pulled in 'images' stanza
				report.Images = images
				if err := enc.Encode(report); err != nil {
					logrus.Warnf("Failed to json encode error %q", err.Error())
				}

				report.Images = nil
				// Pull last ID from list and publish in 'id' stanza.  This maintains previous API contract
				report.ID = images[len(images)-1]
				if err := enc.Encode(report); err != nil {
					logrus.Warnf("Failed to json encode error %q", err.Error())
				}

				flush()
			}
			break loop // break out of for/select infinite loop
		case <-r.Context().Done():
			// Client has closed connection
			break loop // break out of for/select infinite loop
		}
	}
}
