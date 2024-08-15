//go:build !remote

package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/containers/common/pkg/auth"
	DockerClient "github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/docker/docker/api/types/registry"
)

func Auth(w http.ResponseWriter, r *http.Request) {
	var authConfig registry.AuthConfig
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

	loginOpts := &auth.LoginOptions{
		Username:    authConfig.Username,
		Password:    authConfig.Password,
		Stdout:      io.Discard,
		NoWriteBack: true, // to prevent credentials to be written on disk
	}
	if err := auth.Login(r.Context(), sysCtx, loginOpts, []string{authConfig.ServerAddress}); err == nil {
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
