package compat

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/runtime-spec/specs-go"

	"net/http"
	"os"
	"time"

	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Archive(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	switch r.Method {
	case http.MethodPut:
		handlePut(w, r, decoder, runtime)
	case http.MethodGet, http.MethodHead:
		handleHeadOrGet(w, r, decoder, runtime)
	default:
		utils.Error(w, fmt.Sprintf("not implemented, method: %v", r.Method), http.StatusNotImplemented, errors.New(fmt.Sprintf("not implemented, method: %v", r.Method)))
	}
}

func handleHeadOrGet(w http.ResponseWriter, r *http.Request, decoder *schema.Decoder, runtime *libpod.Runtime) {
	query := struct {
		Path string `schema:"path"`
	}{}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, "Bad Request.", http.StatusBadRequest, errors.Wrap(err, "couldn't decode the query"))
		return
	}

	if query.Path == "" {
		utils.Error(w, "Bad Request.", http.StatusBadRequest, errors.New("missing `path` parameter"))
		return
	}

	containerName := utils.GetName(r)

	ctr, err := runtime.LookupContainer(containerName)
	if errors.Cause(err) == define.ErrNoSuchCtr {
		utils.Error(w, "Not found.", http.StatusNotFound, errors.Wrap(err, "the container doesn't exists"))
		return
	} else if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	mountPoint, err := ctr.Mount()
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to mount the container"))
		return
	}

	defer func() {
		if err := ctr.Unmount(true); err != nil {
			logrus.Warnf("failed to unmount container %s: %q", containerName, err)
		}
	}()

	opts := copier.StatOptions{}

	mountPoint, path, err := fixUpMountPointAndPath(runtime, ctr, mountPoint, query.Path)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	stats, err := copier.Stat(mountPoint, "", opts, []string{filepath.Join(mountPoint, path)})
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to get stats about file"))
		return
	}

	if len(stats) <= 0 || len(stats[0].Globbed) <= 0 {
		errs := make([]string, 0, len(stats))

		for _, stat := range stats {
			if stat.Error != "" {
				errs = append(errs, stat.Error)
			}
		}

		utils.Error(w, "Not found.", http.StatusNotFound, fmt.Errorf("file doesn't exist (errs: %q)", strings.Join(errs, ";")))

		return
	}

	statHeader, err := statsToHeader(stats[0].Results[stats[0].Globbed[0]])
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	w.Header().Add("X-Docker-Container-Path-Stat", statHeader)

	if r.Method == http.MethodGet {
		idMappingOpts, err := ctr.IDMappings()
		if err != nil {
			utils.Error(w, "Not found.", http.StatusInternalServerError,
				errors.Wrapf(err, "error getting IDMappingOptions"))
			return
		}

		destOwner := idtools.IDPair{UID: os.Getuid(), GID: os.Getgid()}

		opts := copier.GetOptions{
			UIDMap:             idMappingOpts.UIDMap,
			GIDMap:             idMappingOpts.GIDMap,
			ChownDirs:          &destOwner,
			ChownFiles:         &destOwner,
			KeepDirectoryNames: true,
		}

		w.WriteHeader(http.StatusOK)

		err = copier.Get(mountPoint, "", opts, []string{filepath.Join(mountPoint, path)}, w)
		if err != nil {
			logrus.Error(errors.Wrapf(err, "failed to copy from the %s container path %s", containerName, query.Path))
			return
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func handlePut(w http.ResponseWriter, r *http.Request, decoder *schema.Decoder, runtime *libpod.Runtime) {
	query := struct {
		Path string `schema:"path"`
		// TODO handle params below
		NoOverwriteDirNonDir bool `schema:"noOverwriteDirNonDir"`
		CopyUIDGID           bool `schema:"copyUIDGID"`
	}{}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, "Bad Request.", http.StatusBadRequest, errors.Wrap(err, "couldn't decode the query"))
		return
	}

	ctrName := utils.GetName(r)

	ctr, err := runtime.LookupContainer(ctrName)
	if err != nil {
		utils.Error(w, "Not found", http.StatusNotFound, errors.Wrapf(err, "the %s container doesn't exists", ctrName))
		return
	}

	mountPoint, err := ctr.Mount()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, errors.Wrapf(err, "failed to mount the %s container", ctrName))
		return
	}

	defer func() {
		if err := ctr.Unmount(true); err != nil {
			logrus.Warnf("failed to unmount container %s", ctrName)
		}
	}()

	user, err := getUser(mountPoint, ctr.User())
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	idMappingOpts, err := ctr.IDMappings()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, errors.Wrapf(err, "error getting IDMappingOptions"))
		return
	}

	destOwner := idtools.IDPair{UID: int(user.UID), GID: int(user.GID)}

	opts := copier.PutOptions{
		UIDMap:     idMappingOpts.UIDMap,
		GIDMap:     idMappingOpts.GIDMap,
		ChownDirs:  &destOwner,
		ChownFiles: &destOwner,
	}

	mountPoint, path, err := fixUpMountPointAndPath(runtime, ctr, mountPoint, query.Path)
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)

	err = copier.Put(mountPoint, filepath.Join(mountPoint, path), opts, r.Body)
	if err != nil {
		logrus.Error(errors.Wrapf(err, "failed to copy to the %s container path %s", ctrName, query.Path))
		return
	}
}

func statsToHeader(stats *copier.StatForItem) (string, error) {
	statsDTO := struct {
		Name       string      `json:"name"`
		Size       int64       `json:"size"`
		Mode       os.FileMode `json:"mode"`
		ModTime    time.Time   `json:"mtime"`
		LinkTarget string      `json:"linkTarget"`
	}{
		Name:       filepath.Base(stats.Name),
		Size:       stats.Size,
		Mode:       stats.Mode,
		ModTime:    stats.ModTime,
		LinkTarget: stats.ImmediateTarget,
	}

	jsonBytes, err := json.Marshal(&statsDTO)
	if err != nil {
		return "", errors.Wrap(err, "failed to serialize file stats")
	}

	buff := bytes.NewBuffer(make([]byte, 0, 128))
	base64encoder := base64.NewEncoder(base64.StdEncoding, buff)

	_, err = base64encoder.Write(jsonBytes)
	if err != nil {
		return "", err
	}

	err = base64encoder.Close()
	if err != nil {
		return "", err
	}

	return buff.String(), nil
}

func getUser(mountPoint string, userspec string) (specs.User, error) {
	uid, gid, _, err := chrootuser.GetUser(mountPoint, userspec)
	u := specs.User{
		UID:      uid,
		GID:      gid,
		Username: userspec,
	}

	if !strings.Contains(userspec, ":") {
		groups, err2 := chrootuser.GetAdditionalGroupsForUser(mountPoint, uint64(u.UID))
		if err2 != nil {
			if errors.Cause(err2) != chrootuser.ErrNoSuchUser && err == nil {
				err = err2
			}
		} else {
			u.AdditionalGids = groups
		}
	}

	return u, err
}

func fixUpMountPointAndPath(runtime *libpod.Runtime, ctr *libpod.Container, mountPoint, ctrPath string) (string, string, error) {
	if !filepath.IsAbs(ctrPath) {
		endsWithSep := strings.HasSuffix(ctrPath, string(filepath.Separator))
		ctrPath = filepath.Join(ctr.WorkingDir(), ctrPath)

		if endsWithSep {
			ctrPath = ctrPath + string(filepath.Separator)
		}
	}
	if isVol, volDestName, volName := isVolumeDestName(ctrPath, ctr); isVol { //nolint(gocritic)
		newMountPoint, path, err := pathWithVolumeMount(runtime, volDestName, volName, ctrPath)
		if err != nil {
			return "", "", errors.Wrapf(err, "error getting source path from volume %s", volDestName)
		}

		mountPoint = newMountPoint
		ctrPath = path
	} else if isBindMount, mount := isBindMountDestName(ctrPath, ctr); isBindMount { //nolint(gocritic)
		newMountPoint, path := pathWithBindMountSource(mount, ctrPath)
		mountPoint = newMountPoint
		ctrPath = path
	}

	return mountPoint, ctrPath, nil
}

func isVolumeDestName(path string, ctr *libpod.Container) (bool, string, string) {
	separator := string(os.PathSeparator)

	if filepath.IsAbs(path) {
		path = strings.TrimPrefix(path, separator)
	}

	if path == "" {
		return false, "", ""
	}

	for _, vol := range ctr.Config().NamedVolumes {
		volNamePath := strings.TrimPrefix(vol.Dest, separator)
		if matchVolumePath(path, volNamePath) {
			return true, vol.Dest, vol.Name
		}
	}

	return false, "", ""
}

func pathWithVolumeMount(runtime *libpod.Runtime, volDestName, volName, path string) (string, string, error) {
	destVolume, err := runtime.GetVolume(volName)
	if err != nil {
		return "", "", errors.Wrapf(err, "error getting volume destination %s", volName)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(string(os.PathSeparator), path)
	}

	return destVolume.MountPoint(), strings.TrimPrefix(path, volDestName), err
}

func isBindMountDestName(path string, ctr *libpod.Container) (bool, specs.Mount) {
	separator := string(os.PathSeparator)

	if filepath.IsAbs(path) {
		path = strings.TrimPrefix(path, string(os.PathSeparator))
	}

	if path == "" {
		return false, specs.Mount{}
	}

	for _, m := range ctr.Config().Spec.Mounts {
		if m.Type != "bind" {
			continue
		}

		mDest := strings.TrimPrefix(m.Destination, separator)
		if matchVolumePath(path, mDest) {
			return true, m
		}
	}

	return false, specs.Mount{}
}

func matchVolumePath(path, target string) bool {
	pathStr := filepath.Clean(path)
	target = filepath.Clean(target)

	for len(pathStr) > len(target) && strings.Contains(pathStr, string(os.PathSeparator)) {
		pathStr = pathStr[:strings.LastIndex(pathStr, string(os.PathSeparator))]
	}

	return pathStr == target
}

func pathWithBindMountSource(m specs.Mount, path string) (string, string) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(string(os.PathSeparator), path)
	}

	return m.Source, strings.TrimPrefix(path, m.Destination)
}
