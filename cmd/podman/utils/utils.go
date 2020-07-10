package utils

import (
	"fmt"
	"os"

	"github.com/containers/podman/v2/pkg/domain/entities"
)

// IsDir returns true if the specified path refers to a directory.
func IsDir(path string) bool {
	file, err := os.Stat(path)
	if err != nil {
		return false
	}
	return file.IsDir()
}

// FileExists returns true if path refers to an existing file.
func FileExists(path string) bool {
	file, err := os.Stat(path)
	// All errors return file == nil
	if err != nil {
		return false
	}
	return !file.IsDir()
}

func PrintPodPruneResults(podPruneReports []*entities.PodPruneReport) error {
	var errs OutputErrors
	for _, r := range podPruneReports {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func PrintContainerPruneResults(containerPruneReport *entities.ContainerPruneReport) error {
	var errs OutputErrors
	for k := range containerPruneReport.ID {
		fmt.Println(k)
	}
	for _, v := range containerPruneReport.Err {
		errs = append(errs, v)
	}
	return errs.PrintErrors()
}

func PrintVolumePruneResults(volumePruneReport []*entities.VolumePruneReport) error {
	var errs OutputErrors
	for _, r := range volumePruneReport {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func PrintImagePruneResults(imagePruneReport *entities.ImagePruneReport) error {
	for _, i := range imagePruneReport.Report.Id {
		fmt.Println(i)
	}
	for _, e := range imagePruneReport.Report.Err {
		fmt.Fprint(os.Stderr, e.Error()+"\n")
	}
	if imagePruneReport.Size > 0 {
		fmt.Fprintf(os.Stdout, "Size: %d\n", imagePruneReport.Size)
	}
	return nil
}
