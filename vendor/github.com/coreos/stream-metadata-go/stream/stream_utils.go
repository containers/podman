package stream

import "fmt"

// FormatPrefix describes a stream+architecture combination, intended for prepending to error messages
func (st *Stream) FormatPrefix(archname string) string {
	return fmt.Sprintf("%s/%s", st.Stream, archname)
}

// GetArchitecture loads the architecture-specific builds from a stream,
// with a useful descriptive error message if the architecture is not found.
func (st *Stream) GetArchitecture(archname string) (*Arch, error) {
	archdata, ok := st.Architectures[archname]
	if !ok {
		return nil, fmt.Errorf("stream:%s does not have architecture '%s'", st.Stream, archname)
	}
	return &archdata, nil
}

// GetAliyunRegionImage returns the release data (Image ID and release ID) for a particular
// architecture and region.
func (st *Stream) GetAliyunRegionImage(archname, region string) (*SingleImage, error) {
	starch, err := st.GetArchitecture(archname)
	if err != nil {
		return nil, err
	}
	aliyunimages := starch.Images.Aliyun
	if aliyunimages == nil {
		return nil, fmt.Errorf("%s: No Aliyun images", st.FormatPrefix(archname))
	}
	var regionVal SingleImage
	var ok bool
	if regionVal, ok = aliyunimages.Regions[region]; !ok {
		return nil, fmt.Errorf("%s: No Aliyun images in region %s", st.FormatPrefix(archname), region)
	}

	return &regionVal, nil
}

// GetAliyunImage returns the Aliyun image for a particular architecture and region.
func (st *Stream) GetAliyunImage(archname, region string) (string, error) {
	regionVal, err := st.GetAliyunRegionImage(archname, region)
	if err != nil {
		return "", err
	}
	return regionVal.Image, nil
}

// GetAwsRegionImage returns the release data (AMI and release ID) for a particular
// architecture and region.
func (st *Stream) GetAwsRegionImage(archname, region string) (*SingleImage, error) {
	starch, err := st.GetArchitecture(archname)
	if err != nil {
		return nil, err
	}
	awsimages := starch.Images.Aws
	if awsimages == nil {
		return nil, fmt.Errorf("%s: No AWS images", st.FormatPrefix(archname))
	}
	var regionVal SingleImage
	var ok bool
	if regionVal, ok = awsimages.Regions[region]; !ok {
		return nil, fmt.Errorf("%s: No AWS images in region %s", st.FormatPrefix(archname), region)
	}

	return &regionVal, nil
}

// GetAMI returns the AWS machine image for a particular architecture and region.
func (st *Stream) GetAMI(archname, region string) (string, error) {
	regionVal, err := st.GetAwsRegionImage(archname, region)
	if err != nil {
		return "", err
	}
	return regionVal.Image, nil
}

// QueryDisk finds the singleton disk artifact for a given format and architecture.
func (st *Stream) QueryDisk(architectureName, artifactName, formatName string) (*Artifact, error) {
	arch, err := st.GetArchitecture(architectureName)
	if err != nil {
		return nil, err
	}
	artifacts := arch.Artifacts[artifactName]
	if artifacts.Release == "" {
		return nil, fmt.Errorf("%s: artifact '%s' not found", st.FormatPrefix(architectureName), artifactName)
	}
	format := artifacts.Formats[formatName]
	if format.Disk == nil {
		return nil, fmt.Errorf("%s: artifact '%s' format '%s' disk not found", st.FormatPrefix(architectureName), artifactName, formatName)
	}

	return format.Disk, nil
}
