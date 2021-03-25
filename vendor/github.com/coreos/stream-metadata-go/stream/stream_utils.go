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

// GetAwsRegionImage returns the release data (AMI and release ID) for a particular
// architecture and region.
func (st *Stream) GetAwsRegionImage(archname, region string) (*AwsRegionImage, error) {
	starch, err := st.GetArchitecture(archname)
	if err != nil {
		return nil, err
	}
	awsimages := starch.Images.Aws
	if awsimages == nil {
		return nil, fmt.Errorf("%s: No AWS images", st.FormatPrefix(archname))
	}
	var regionVal AwsRegionImage
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
