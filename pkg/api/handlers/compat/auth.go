package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	DockerClient "github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	api "github.com/containers/podman/v3/pkg/api/types"
	"github.com/containers/podman/v3/pkg/domain/entities"
	docker "github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

func stripAddressOfScheme(address string) string {
	for _, s := range []string{"https", "http"} {
		address = strings.TrimPrefix(address, s+"://")
	}
	return address
}

func Auth(w http.ResponseWriter, r *http.Request) {
	var authConfig docker.AuthConfig
	err := json.NewDecoder(r.Body).Decode(&authConfig)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "failed to parse request"))
		return
	}

	skipTLS := types.NewOptionalBool(false)
	if strings.HasPrefix(authConfig.ServerAddress, "https://localhost/") || strings.HasPrefix(authConfig.ServerAddress, "https://localhost:") || strings.HasPrefix(authConfig.ServerAddress, "localhost:") {
		// support for local testing
		skipTLS = types.NewOptionalBool(true)
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	sysCtx := runtime.SystemContext()
	sysCtx.DockerInsecureSkipTLSVerify = skipTLS

	fmt.Println("Authenticating with existing credentials...")
	registry := stripAddressOfScheme(authConfig.ServerAddress)
	if err := DockerClient.CheckAuth(context.Background(), sysCtx, authConfig.Username, authConfig.Password, registry); err == nil {
		utils.WriteResponse(w, http.StatusOK, entities.AuthReport{
			IdentityToken: "",
			Status:        "Login Succeeded",
		})
	} else {
		var msg string

		var unauthErr DockerClient.ErrUnauthorizedForCredentials
		if errors.As(err, &unauthErr) {
			msg = "401 Unauthorized"
		} else {
			msg = err.Error()
		}

		utils.WriteResponse(w, http.StatusInternalServerError, struct {
			Message string `json:"message"`
		}{
			Message: "login attempt to " + authConfig.ServerAddress + " failed with status: " + msg,
		})
	}
}
