package main

import (
	"strings"
	"testing"
)

func expect(t *testing.T, expected string, returned string) {
	if strings.Compare(returned, expected) != 0 {
		t.Fatalf("Expected(%d): `%s`\nGot(%d): `%s`\n",
			len(expected), expected,
			len(returned), returned)
	}
}

func TestIDLParser(t *testing.T) {
	pkgname, b, err := generateTemplate(`
# Interface to jump a spacecraft to another point in space. The 
# FTL Drive is the propulsion system to achieve faster-than-light
# travel through space. A ship making a properly calculated
# jump can arrive safely in planetary orbit, or alongside other
# ships or spaceborne objects.
interface org.example.ftl

# The current state of the FTL drive and the amount of fuel
# available to jump.
type DriveCondition (
  state: (idle, spooling, busy),
  booster: bool,
  active_engines: [](id: int, state: bool),
  tylium_level: int
)

# Speed, trajectory and jump duration is calculated prior to
# activating the FTL drive.
type DriveConfiguration (
  speed: int,
  trajectory: int,
  duration: int
)

# The galactic coordinates use the Sun as the origin. Galactic
# longitude is measured with primary direction from the Sun to
# the center of the galaxy in the galactic plane, while the
# galactic latitude measures the angle of the object above the
# galactic plane.
type Coordinate (
  longitude: float,
  latitude: float,
  distance: int
)

# Monitor the drive. The method will reply with an update whenever
# the drive's state changes
method Monitor() -> (condition: DriveCondition)

# Calculate the drive's jump parameters from the current
# position to the target position in the galaxy
method CalculateConfiguration(
  current: Coordinate,
  target: Coordinate
) -> (configuration: DriveConfiguration)

# Jump to the calculated point in space
method Jump(configuration: DriveConfiguration) -> ()

# There is not enough tylium to jump with the given parameters
error NotEnoughEnergy ()

# The supplied parameters are outside the supported range
error ParameterOutOfRange (field: string)

# some more coverage
method Foo(interface: string) -> (ret: (go: string, switch: bool, more: (t:bool, f:bool)))

# some more coverage
method TestMap(map: [string]string) -> (map: [string](i: int, val: string))
method TestSet(set: [string]()) -> (set: [string]())
method TestObject(object: object) -> (object: object)
	`)

	if err != nil {
		t.Fatalf("Error parsing %v", err)
	}
	expect(t, "orgexampleftl", pkgname)
	if len(b) <= 0 {
		t.Fatal("No generated go source")
	}
	// FIXME: compare b.String() against expected output
}
