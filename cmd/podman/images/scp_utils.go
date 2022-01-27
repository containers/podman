package images

import (
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
)

// parseImageSCPArg returns the valid connection, and source/destination data based off of the information provided by the user
// arg is a string containing one of the cli arguments returned is a filled out source/destination options structs as well as a connections array and an error if applicable
func parseImageSCPArg(arg string) (*entities.ImageScpOptions, []string, error) {
	location := entities.ImageScpOptions{}
	var err error
	cliConnections := []string{}

	switch {
	case strings.Contains(arg, "@localhost::"): // image transfer between users
		location.User = strings.Split(arg, "@")[0]
		location, err = validateImagePortion(location, arg)
		if err != nil {
			return nil, nil, err
		}
		cliConnections = append(cliConnections, arg)
	case strings.Contains(arg, "::"):
		location, err = validateImagePortion(location, arg)
		if err != nil {
			return nil, nil, err
		}
		location.Remote = true
		cliConnections = append(cliConnections, arg)
	default:
		location.Image = arg
	}
	return &location, cliConnections, nil
}

// validateImagePortion is a helper function to validate the image name in an SCP argument
func validateImagePortion(location entities.ImageScpOptions, arg string) (entities.ImageScpOptions, error) {
	if remoteArgLength(arg, 1) > 0 {
		err := validateImageName(strings.Split(arg, "::")[1])
		if err != nil {
			return location, err
		}
		location.Image = strings.Split(arg, "::")[1] // this will get checked/set again once we validate connections
	}
	return location, nil
}

// validateSCPArgs takes the array of source and destination options and checks for common errors
func validateSCPArgs(locations []*entities.ImageScpOptions) (bool, error) {
	if len(locations) > 2 {
		return false, errors.Wrapf(define.ErrInvalidArg, "cannot specify more than two arguments")
	}
	switch {
	case len(locations[0].Image) > 0 && len(locations[1].Image) > 0:
		return false, errors.Wrapf(define.ErrInvalidArg, "cannot specify an image rename")
	case len(locations[0].Image) == 0 && len(locations[1].Image) == 0:
		return false, errors.Wrapf(define.ErrInvalidArg, "a source image must be specified")
	case len(locations[0].Image) == 0 && len(locations[1].Image) != 0:
		if locations[0].Remote && locations[1].Remote {
			return true, nil // we need to flip the cliConnections array so the save/load connections are in the right place
		}
	}
	return false, nil
}

// validateImageName makes sure that the image given is valid and no injections are occurring
// we simply use this for error checking, bot setting the image
func validateImageName(input string) error {
	// ParseNormalizedNamed transforms a shortname image into its
	// full name reference so busybox => docker.io/library/busybox
	// we want to keep our shortnames, so only return an error if
	// we cannot parse what the user has given us
	_, err := reference.ParseNormalizedNamed(input)
	return err
}

// remoteArgLength is a helper function to simplify the extracting of host argument data
// returns an int which contains the length of a specified index in a host::image string
func remoteArgLength(input string, side int) int {
	if strings.Contains(input, "::") {
		return len((strings.Split(input, "::"))[side])
	}
	return -1
}
