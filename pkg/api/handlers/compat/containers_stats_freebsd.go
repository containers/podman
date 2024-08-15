//go:build !remote

package compat

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containers/podman/v5/pkg/api/handlers/utils"
)

const DefaultStatsPeriod = 5 * time.Second

func StatsContainer(w http.ResponseWriter, r *http.Request) {
	utils.Error(w, http.StatusBadRequest, fmt.Errorf("compat.StatsContainer not supported on FreeBSD"))
}
