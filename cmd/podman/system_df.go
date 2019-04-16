package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	units "github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	dfSystemCommand     cliconfig.SystemDfValues
	dfSystemDescription = `
	podman system df

	Show podman disk usage
	`
	_dfSystemCommand = &cobra.Command{
		Use:   "df",
		Args:  noSubArgs,
		Short: "Show podman disk usage",
		Long:  dfSystemDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			dfSystemCommand.GlobalFlags = MainGlobalOpts
			dfSystemCommand.Remote = remoteclient
			return dfSystemCmd(&dfSystemCommand)
		},
	}
)

type dfMetaData struct {
	images                   []*image.Image
	containers               []*libpod.Container
	activeContainers         map[string]*libpod.Container
	imagesUsedbyCtrMap       map[string][]*libpod.Container
	imagesUsedbyActiveCtr    map[string][]*libpod.Container
	volumes                  []*libpod.Volume
	volumeUsedByContainerMap map[string][]*libpod.Container
}

type systemDfDiskUsage struct {
	Type        string
	Total       int
	Active      int
	Size        string
	Reclaimable string
}

type imageVerboseDiskUsage struct {
	Repository string
	Tag        string
	ImageID    string
	Created    string
	Size       string
	SharedSize string
	UniqueSize string
	Containers int
}

type containerVerboseDiskUsage struct {
	ContainerID  string
	Image        string
	Command      string
	LocalVolumes int
	Size         string
	Created      string
	Status       string
	Names        string
}

type volumeVerboseDiskUsage struct {
	VolumeName string
	Links      int
	Size       string
}

const systemDfDefaultFormat string = "table {{.Type}}\t{{.Total}}\t{{.Active}}\t{{.Size}}\t{{.Reclaimable}}"
const imageVerboseFormat string = "table {{.Repository}}\t{{.Tag}}\t{{.ImageID}}\t{{.Created}}\t{{.Size}}\t{{.SharedSize}}\t{{.UniqueSize}}\t{{.Containers}}"
const containerVerboseFormat string = "table {{.ContainerID}}\t{{.Image}}\t{{.Command}}\t{{.LocalVolumes}}\t{{.Size}}\t{{.Created}}\t{{.Status}}\t{{.Names}}"
const volumeVerboseFormat string = "table {{.VolumeName}}\t{{.Links}}\t{{.Size}}"

func init() {
	dfSystemCommand.Command = _dfSystemCommand
	dfSystemCommand.SetUsageTemplate(UsageTemplate())
	flags := dfSystemCommand.Flags()
	flags.BoolVarP(&dfSystemCommand.Verbose, "verbose", "v", false, "Show detailed information on space usage")
	flags.StringVar(&dfSystemCommand.Format, "format", "", "Pretty-print images using a Go template")
}

func dfSystemCmd(c *cliconfig.SystemDfValues) error {
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "Could not get runtime")
	}
	defer runtime.Shutdown(false)

	ctx := getContext()

	metaData, err := getDfMetaData(ctx, runtime)
	if err != nil {
		return errors.Wrapf(err, "error getting disk usage data")
	}

	if c.Verbose {
		err := verboseOutput(ctx, metaData)
		if err != nil {
			return err
		}
		return nil
	}

	systemDfDiskUsages, err := getDiskUsage(ctx, runtime, metaData)
	if err != nil {
		return errors.Wrapf(err, "error getting output of system df")
	}
	format := systemDfDefaultFormat
	if c.Format != "" {
		format = strings.Replace(c.Format, `\t`, "\t", -1)
	}
	generateSysDfOutput(systemDfDiskUsages, format)
	return nil
}

func generateSysDfOutput(systemDfDiskUsages []systemDfDiskUsage, format string) {
	var systemDfHeader = map[string]string{
		"Type":        "TYPE",
		"Total":       "TOTAL",
		"Active":      "ACTIVE",
		"Size":        "SIZE",
		"Reclaimable": "RECLAIMABLE",
	}
	out := formats.StdoutTemplateArray{Output: systemDfDiskUsageToGeneric(systemDfDiskUsages), Template: format, Fields: systemDfHeader}
	formats.Writer(out).Out()
}

func getDiskUsage(ctx context.Context, runtime *libpod.Runtime, metaData dfMetaData) ([]systemDfDiskUsage, error) {
	imageDiskUsage, err := getImageDiskUsage(ctx, metaData.images, metaData.imagesUsedbyCtrMap, metaData.imagesUsedbyActiveCtr)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting disk usage of images")
	}
	containerDiskUsage, err := getContainerDiskUsage(metaData.containers, metaData.activeContainers)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting disk usage of containers")
	}
	volumeDiskUsage, err := getVolumeDiskUsage(metaData.volumes, metaData.volumeUsedByContainerMap)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting disk usage of volumess")
	}

	systemDfDiskUsages := []systemDfDiskUsage{imageDiskUsage, containerDiskUsage, volumeDiskUsage}
	return systemDfDiskUsages, nil
}

func getDfMetaData(ctx context.Context, runtime *libpod.Runtime) (dfMetaData, error) {
	var metaData dfMetaData
	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		return metaData, errors.Wrapf(err, "unable to get images")
	}
	containers, err := runtime.GetAllContainers()
	if err != nil {
		return metaData, errors.Wrapf(err, "error getting all containers")
	}
	volumes, err := runtime.GetAllVolumes()
	if err != nil {
		return metaData, errors.Wrap(err, "error getting all volumes")
	}
	activeContainers, err := activeContainers(containers)
	if err != nil {
		return metaData, errors.Wrapf(err, "error getting active containers")
	}
	imagesUsedbyCtrMap, imagesUsedbyActiveCtr, err := imagesUsedbyCtr(containers, activeContainers)
	if err != nil {
		return metaData, errors.Wrapf(err, "error getting getting images used by containers")
	}
	metaData = dfMetaData{
		images:                   images,
		containers:               containers,
		activeContainers:         activeContainers,
		imagesUsedbyCtrMap:       imagesUsedbyCtrMap,
		imagesUsedbyActiveCtr:    imagesUsedbyActiveCtr,
		volumes:                  volumes,
		volumeUsedByContainerMap: volumeUsedByContainer(containers),
	}
	return metaData, nil
}

func imageUniqueSize(ctx context.Context, images []*image.Image) (map[string]uint64, error) {
	imgUniqueSizeMap := make(map[string]uint64)
	for _, img := range images {
		parentImg := img
		for {
			next, err := parentImg.GetParent()
			if err != nil {
				return nil, errors.Wrapf(err, "error getting parent of image %s", parentImg.ID())
			}
			if next == nil {
				break
			}
			parentImg = next
		}
		imgSize, err := img.Size(ctx)
		if err != nil {
			return nil, err
		}
		if img.ID() == parentImg.ID() {
			imgUniqueSizeMap[img.ID()] = *imgSize
		} else {
			parentImgSize, err := parentImg.Size(ctx)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting size of parent image %s", parentImg.ID())
			}
			imgUniqueSizeMap[img.ID()] = *imgSize - *parentImgSize
		}
	}
	return imgUniqueSizeMap, nil
}

func getImageDiskUsage(ctx context.Context, images []*image.Image, imageUsedbyCintainerMap map[string][]*libpod.Container, imageUsedbyActiveContainerMap map[string][]*libpod.Container) (systemDfDiskUsage, error) {
	var (
		numberOfImages       int
		sumSize              uint64
		numberOfActiveImages int
		unreclaimableSize    uint64
		imageDiskUsage       systemDfDiskUsage
		reclaimableStr       string
	)

	imgUniqueSizeMap, err := imageUniqueSize(ctx, images)
	if err != nil {
		return imageDiskUsage, errors.Wrapf(err, "error getting unique size of images")
	}

	for _, img := range images {

		unreclaimableSize += imageUsedSize(img, imgUniqueSizeMap, imageUsedbyCintainerMap, imageUsedbyActiveContainerMap)

		isParent, err := img.IsParent()
		if err != nil {
			return imageDiskUsage, err
		}
		parent, err := img.GetParent()
		if err != nil {
			return imageDiskUsage, errors.Wrapf(err, "error getting parent of image %s", img.ID())
		}
		if isParent && parent != nil {
			continue
		}
		numberOfImages++
		if _, isActive := imageUsedbyCintainerMap[img.ID()]; isActive {
			numberOfActiveImages++
		}

		if !isParent {
			size, err := img.Size(ctx)
			if err != nil {
				return imageDiskUsage, errors.Wrapf(err, "error getting disk usage of image %s", img.ID())
			}
			sumSize += *size
		}

	}
	sumSizeStr := units.HumanSizeWithPrecision(float64(sumSize), 3)
	reclaimable := sumSize - unreclaimableSize
	if sumSize != 0 {
		reclaimableStr = fmt.Sprintf("%s (%v%%)", units.HumanSizeWithPrecision(float64(reclaimable), 3), 100*reclaimable/sumSize)
	} else {
		reclaimableStr = fmt.Sprintf("%s (%v%%)", units.HumanSizeWithPrecision(float64(reclaimable), 3), 0)
	}
	imageDiskUsage = systemDfDiskUsage{
		Type:        "Images",
		Total:       numberOfImages,
		Active:      numberOfActiveImages,
		Size:        sumSizeStr,
		Reclaimable: reclaimableStr,
	}
	return imageDiskUsage, nil
}

func imageUsedSize(img *image.Image, imgUniqueSizeMap map[string]uint64, imageUsedbyCintainerMap map[string][]*libpod.Container, imageUsedbyActiveContainerMap map[string][]*libpod.Container) uint64 {
	var usedSize uint64
	imgUnique := imgUniqueSizeMap[img.ID()]
	if _, isCtrActive := imageUsedbyActiveContainerMap[img.ID()]; isCtrActive {
		return imgUnique
	}
	containers := imageUsedbyCintainerMap[img.ID()]
	for _, ctr := range containers {
		if len(ctr.UserVolumes()) > 0 {
			usedSize += imgUnique
			return usedSize
		}
	}
	return usedSize
}

func imagesUsedbyCtr(containers []*libpod.Container, activeContainers map[string]*libpod.Container) (map[string][]*libpod.Container, map[string][]*libpod.Container, error) {
	imgCtrMap := make(map[string][]*libpod.Container)
	imgActiveCtrMap := make(map[string][]*libpod.Container)
	for _, ctr := range containers {
		imgID, _ := ctr.Image()
		imgCtrMap[imgID] = append(imgCtrMap[imgID], ctr)
		if _, isActive := activeContainers[ctr.ID()]; isActive {
			imgActiveCtrMap[imgID] = append(imgActiveCtrMap[imgID], ctr)
		}
	}
	return imgCtrMap, imgActiveCtrMap, nil
}

func getContainerDiskUsage(containers []*libpod.Container, activeContainers map[string]*libpod.Container) (systemDfDiskUsage, error) {
	var (
		sumSize           int64
		unreclaimableSize int64
		reclaimableStr    string
	)
	for _, ctr := range containers {
		size, err := ctr.RWSize()
		if err != nil {
			return systemDfDiskUsage{}, errors.Wrapf(err, "error getting size of container %s", ctr.ID())
		}
		sumSize += size
	}
	for _, activeCtr := range activeContainers {
		size, err := activeCtr.RWSize()
		if err != nil {
			return systemDfDiskUsage{}, errors.Wrapf(err, "error getting size of active container %s", activeCtr.ID())
		}
		unreclaimableSize += size
	}
	if sumSize == 0 {
		reclaimableStr = fmt.Sprintf("%s (%v%%)", units.HumanSizeWithPrecision(0, 3), 0)
	} else {
		reclaimable := sumSize - unreclaimableSize
		reclaimableStr = fmt.Sprintf("%s (%v%%)", units.HumanSizeWithPrecision(float64(reclaimable), 3), 100*reclaimable/sumSize)
	}
	containerDiskUsage := systemDfDiskUsage{
		Type:        "Containers",
		Total:       len(containers),
		Active:      len(activeContainers),
		Size:        units.HumanSizeWithPrecision(float64(sumSize), 3),
		Reclaimable: reclaimableStr,
	}
	return containerDiskUsage, nil
}

func ctrIsActive(ctr *libpod.Container) (bool, error) {
	state, err := ctr.State()
	if err != nil {
		return false, err
	}
	return state == libpod.ContainerStatePaused || state == libpod.ContainerStateRunning, nil
}

func activeContainers(containers []*libpod.Container) (map[string]*libpod.Container, error) {
	activeContainers := make(map[string]*libpod.Container)
	for _, aCtr := range containers {
		isActive, err := ctrIsActive(aCtr)
		if err != nil {
			return nil, err
		}
		if isActive {
			activeContainers[aCtr.ID()] = aCtr
		}
	}
	return activeContainers, nil
}

func getVolumeDiskUsage(volumes []*libpod.Volume, volumeUsedByContainerMap map[string][]*libpod.Container) (systemDfDiskUsage, error) {
	var (
		sumSize           int64
		unreclaimableSize int64
		reclaimableStr    string
	)
	for _, volume := range volumes {
		size, err := volumeSize(volume)
		if err != nil {
			return systemDfDiskUsage{}, errors.Wrapf(err, "error getting size of volime %s", volume.Name())
		}
		sumSize += size
		if _, exist := volumeUsedByContainerMap[volume.Name()]; exist {
			unreclaimableSize += size
		}
	}
	reclaimable := sumSize - unreclaimableSize
	if sumSize != 0 {
		reclaimableStr = fmt.Sprintf("%s (%v%%)", units.HumanSizeWithPrecision(float64(reclaimable), 3), 100*reclaimable/sumSize)
	} else {
		reclaimableStr = fmt.Sprintf("%s (%v%%)", units.HumanSizeWithPrecision(float64(reclaimable), 3), 0)
	}
	volumesDiskUsage := systemDfDiskUsage{
		Type:        "Local Volumes",
		Total:       len(volumes),
		Active:      len(volumeUsedByContainerMap),
		Size:        units.HumanSizeWithPrecision(float64(sumSize), 3),
		Reclaimable: reclaimableStr,
	}
	return volumesDiskUsage, nil
}

func volumeUsedByContainer(containers []*libpod.Container) map[string][]*libpod.Container {
	volumeUsedByContainerMap := make(map[string][]*libpod.Container)
	for _, ctr := range containers {

		ctrVolumes := ctr.UserVolumes()
		for _, ctrVolume := range ctrVolumes {
			volumeUsedByContainerMap[ctrVolume] = append(volumeUsedByContainerMap[ctrVolume], ctr)
		}
	}
	return volumeUsedByContainerMap
}

func volumeSize(volume *libpod.Volume) (int64, error) {
	var size int64
	err := filepath.Walk(volume.MountPoint(), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func getImageVerboseDiskUsage(ctx context.Context, images []*image.Image, imagesUsedbyCtr map[string][]*libpod.Container) ([]imageVerboseDiskUsage, error) {
	var imagesVerboseDiskUsage []imageVerboseDiskUsage
	imgUniqueSizeMap, err := imageUniqueSize(ctx, images)
	if err != nil {
		return imagesVerboseDiskUsage, errors.Wrapf(err, "error getting unique size of images")
	}
	for _, img := range images {
		isParent, err := img.IsParent()
		if err != nil {
			return imagesVerboseDiskUsage, errors.Wrapf(err, "error checking if %s is a parent images", img.ID())
		}
		parent, err := img.GetParent()
		if err != nil {
			return imagesVerboseDiskUsage, errors.Wrapf(err, "error getting parent of image %s", img.ID())
		}
		if isParent && parent != nil {
			continue
		}
		size, err := img.Size(ctx)
		if err != nil {
			return imagesVerboseDiskUsage, errors.Wrapf(err, "error getting size of image %s", img.ID())
		}
		numberOfContainers := 0
		if ctrs, exist := imagesUsedbyCtr[img.ID()]; exist {
			numberOfContainers = len(ctrs)
		}
		var repo string
		var tag string
		if len(img.Names()) == 0 {
			repo = "<none>"
			tag = "<none>"
		}
		repopairs, err := image.ReposToMap([]string{img.Names()[0]})
		if err != nil {
			logrus.Errorf("error finding tag/digest for %s", img.ID())
		}
		for reponame, tags := range repopairs {
			for _, tagname := range tags {
				repo = reponame
				tag = tagname
			}
		}

		imageVerbosedf := imageVerboseDiskUsage{
			Repository: repo,
			Tag:        tag,
			ImageID:    shortID(img.ID()),
			Created:    fmt.Sprintf("%s ago", units.HumanDuration(time.Since((img.Created().Local())))),
			Size:       units.HumanSizeWithPrecision(float64(*size), 3),
			SharedSize: units.HumanSizeWithPrecision(float64(*size-imgUniqueSizeMap[img.ID()]), 3),
			UniqueSize: units.HumanSizeWithPrecision(float64(imgUniqueSizeMap[img.ID()]), 3),
			Containers: numberOfContainers,
		}
		imagesVerboseDiskUsage = append(imagesVerboseDiskUsage, imageVerbosedf)
	}
	return imagesVerboseDiskUsage, nil
}

func getContainerVerboseDiskUsage(containers []*libpod.Container) (containersVerboseDiskUsage []containerVerboseDiskUsage, err error) {
	for _, ctr := range containers {
		imgID, _ := ctr.Image()
		size, err := ctr.RWSize()
		if err != nil {
			return containersVerboseDiskUsage, errors.Wrapf(err, "error getting size of container %s", ctr.ID())
		}
		state, err := ctr.State()
		if err != nil {
			return containersVerboseDiskUsage, errors.Wrapf(err, "error getting the state of container %s", ctr.ID())
		}

		ctrVerboseData := containerVerboseDiskUsage{
			ContainerID:  shortID(ctr.ID()),
			Image:        shortImageID(imgID),
			Command:      strings.Join(ctr.Command(), " "),
			LocalVolumes: len(ctr.UserVolumes()),
			Size:         units.HumanSizeWithPrecision(float64(size), 3),
			Created:      fmt.Sprintf("%s ago", units.HumanDuration(time.Since(ctr.CreatedTime().Local()))),
			Status:       state.String(),
			Names:        ctr.Name(),
		}
		containersVerboseDiskUsage = append(containersVerboseDiskUsage, ctrVerboseData)

	}
	return containersVerboseDiskUsage, nil
}

func getVolumeVerboseDiskUsage(volumes []*libpod.Volume, volumeUsedByContainerMap map[string][]*libpod.Container) (volumesVerboseDiskUsage []volumeVerboseDiskUsage, err error) {
	for _, vol := range volumes {
		volSize, err := volumeSize(vol)
		if err != nil {
			return volumesVerboseDiskUsage, errors.Wrapf(err, "error getting size of volume %s", vol.Name())
		}
		links := 0
		if linkCtr, exist := volumeUsedByContainerMap[vol.Name()]; exist {
			links = len(linkCtr)
		}
		volumeVerboseData := volumeVerboseDiskUsage{
			VolumeName: vol.Name(),
			Links:      links,
			Size:       units.HumanSizeWithPrecision(float64(volSize), 3),
		}
		volumesVerboseDiskUsage = append(volumesVerboseDiskUsage, volumeVerboseData)
	}
	return volumesVerboseDiskUsage, nil
}

func imagesVerboseOutput(ctx context.Context, metaData dfMetaData) error {
	var imageVerboseHeader = map[string]string{
		"Repository": "REPOSITORY",
		"Tag":        "TAG",
		"ImageID":    "IMAGE ID",
		"Created":    "CREATED",
		"Size":       "SIZE",
		"SharedSize": "SHARED SIZE",
		"UniqueSize": "UNQUE SIZE",
		"Containers": "CONTAINERS",
	}
	imagesVerboseDiskUsage, err := getImageVerboseDiskUsage(ctx, metaData.images, metaData.imagesUsedbyCtrMap)
	if err != nil {
		return errors.Wrapf(err, "error getting verbose output of images")
	}
	os.Stderr.WriteString("Images space usage:\n\n")
	out := formats.StdoutTemplateArray{Output: systemDfImageVerboseDiskUsageToGeneric(imagesVerboseDiskUsage), Template: imageVerboseFormat, Fields: imageVerboseHeader}
	formats.Writer(out).Out()
	return nil
}

func containersVerboseOutput(ctx context.Context, metaData dfMetaData) error {
	var containerVerboseHeader = map[string]string{
		"ContainerID":  "CONTAINER ID ",
		"Image":        "IMAGE",
		"Command":      "COMMAND",
		"LocalVolumes": "LOCAL VOLUMES",
		"Size":         "SIZE",
		"Created":      "CREATED",
		"Status":       "STATUS",
		"Names":        "NAMES",
	}
	containersVerboseDiskUsage, err := getContainerVerboseDiskUsage(metaData.containers)
	if err != nil {
		return errors.Wrapf(err, "error getting verbose output of containers")
	}
	os.Stderr.WriteString("\nContainers space usage:\n\n")
	out := formats.StdoutTemplateArray{Output: systemDfContainerVerboseDiskUsageToGeneric(containersVerboseDiskUsage), Template: containerVerboseFormat, Fields: containerVerboseHeader}
	formats.Writer(out).Out()
	return nil
}

func volumesVerboseOutput(ctx context.Context, metaData dfMetaData) error {
	var volumeVerboseHeader = map[string]string{
		"VolumeName": "VOLUME NAME",
		"Links":      "LINKS",
		"Size":       "SIZE",
	}
	volumesVerboseDiskUsage, err := getVolumeVerboseDiskUsage(metaData.volumes, metaData.volumeUsedByContainerMap)
	if err != nil {
		return errors.Wrapf(err, "error getting verbose ouput of volumes")
	}
	os.Stderr.WriteString("\nLocal Volumes space usage:\n\n")
	out := formats.StdoutTemplateArray{Output: systemDfVolumeVerboseDiskUsageToGeneric(volumesVerboseDiskUsage), Template: volumeVerboseFormat, Fields: volumeVerboseHeader}
	formats.Writer(out).Out()
	return nil
}

func verboseOutput(ctx context.Context, metaData dfMetaData) error {
	if err := imagesVerboseOutput(ctx, metaData); err != nil {
		return err
	}
	if err := containersVerboseOutput(ctx, metaData); err != nil {
		return err
	}
	if err := volumesVerboseOutput(ctx, metaData); err != nil {
		return err
	}
	return nil
}

func systemDfDiskUsageToGeneric(diskUsages []systemDfDiskUsage) (out []interface{}) {
	for _, usage := range diskUsages {
		out = append(out, interface{}(usage))
	}
	return out
}

func systemDfImageVerboseDiskUsageToGeneric(diskUsages []imageVerboseDiskUsage) (out []interface{}) {
	for _, usage := range diskUsages {
		out = append(out, interface{}(usage))
	}
	return out
}

func systemDfContainerVerboseDiskUsageToGeneric(diskUsages []containerVerboseDiskUsage) (out []interface{}) {
	for _, usage := range diskUsages {
		out = append(out, interface{}(usage))
	}
	return out
}

func systemDfVolumeVerboseDiskUsageToGeneric(diskUsages []volumeVerboseDiskUsage) (out []interface{}) {
	for _, usage := range diskUsages {
		out = append(out, interface{}(usage))
	}
	return out
}

func shortImageID(id string) string {
	const imageIDTruncLength int = 4
	if len(id) > imageIDTruncLength {
		return id[:imageIDTruncLength]
	}
	return id
}
