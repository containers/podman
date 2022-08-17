package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	DockerClient "github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	docker "github.com/docker/docker/api/types"
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
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse request: %w", err))
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
	if err := DockerClient.CheckAuth(r.Context(), sysCtx, authConfig.Username, authConfig.Password, registry); err == nil {
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
