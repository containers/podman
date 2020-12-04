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

func PrintPodPruneResults(podPruneReports []*entities.PodPruneReport, heading bool) error {
	var errs OutputErrors
	if heading && len(podPruneReports) > 0 {
		fmt.Println("Deleted Pods")
	}
	for _, r := range podPruneReports {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func PrintContainerPruneResults(containerPruneReport *entities.ContainerPruneReport, heading bool) error {
	var errs OutputErrors
	if heading && (len(containerPruneReport.ID) > 0 || len(containerPruneReport.Err) > 0) {
		fmt.Println("Deleted Containers")
	}
	for k := range containerPruneReport.ID {
		fmt.Println(k)
	}
	for _, v := range containerPruneReport.Err {
		errs = append(errs, v)
	}
	return errs.PrintErrors()
}

func PrintVolumePruneResults(volumePruneReport []*entities.VolumePruneReport, heading bool) error {
	var errs OutputErrors
	if heading && len(volumePruneReport) > 0 {
		fmt.Println("Deleted Volumes")
	}
	for _, r := range volumePruneReport {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func PrintImagePruneResults(imagePruneReport *entities.ImagePruneReport, heading bool) error {
	if heading && (len(imagePruneReport.Report.Id) > 0 || len(imagePruneReport.Report.Err) > 0) {
		fmt.Println("Deleted Images")
	}
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
