package entities

import (
	"net/url"
	"strings"
)

// Identifier interface allows filters to access ID() of object
type Identifier interface {
	Id() string
}

// Named interface allows filters to access Name() of object
type Named interface {
	Name() string
}

// Named interface allows filters to access Name() of object
type Names interface {
	Names() []string
}

// IDOrName interface allows filters to access ID() or Name() of object
type IDOrNamed interface {
	Identifier
	Named
}

// IDOrName interface allows filters to access ID() or Names() of object
type IDOrNames interface {
	Identifier
	Names
}

type ImageFilter func(Image) bool
type VolumeFilter func(Volume) bool
type ContainerFilter func(Container) bool

func CompileImageFilters(filters url.Values) ImageFilter {
	var fns []interface{}

	for name, targets := range filters {
		switch name {
		case "id":
			fns = append(fns, FilterIDFn(targets))
		case "name":
			fns = append(fns, FilterNamesFn(targets))
		case "idOrName":
			fns = append(fns, FilterIDOrNameFn(targets))
		}
	}

	return func(image Image) bool {
		for _, fn := range fns {
			if !fn.(ImageFilter)(image) {
				return false
			}
		}
		return true
	}
}

func CompileContainerFilters(filters url.Values) ContainerFilter {
	var fns []interface{}

	for name, targets := range filters {
		switch name {
		case "id":
			fns = append(fns, FilterIDFn(targets))
		case "name":
			fns = append(fns, FilterNameFn(targets))
		case "idOrName":
			fns = append(fns, FilterIDOrNameFn(targets))
		}
	}

	return func(ctnr Container) bool {
		for _, fn := range fns {
			if !fn.(ContainerFilter)(ctnr) {
				return false
			}
		}
		return true
	}
}

func CompileVolumeFilters(filters url.Values) VolumeFilter {
	var fns []interface{}

	for name, targets := range filters {
		if name == "id" {
			fns = append(fns, FilterIDFn(targets))
		}
	}

	return func(volume Volume) bool {
		for _, fn := range fns {
			if !fn.(VolumeFilter)(volume) {
				return false
			}
		}
		return true
	}
}

func FilterIDFn(id []string) func(Identifier) bool {
	return func(obj Identifier) bool {
		for _, v := range id {
			if strings.Contains(obj.Id(), v) {
				return true
			}
		}
		return false
	}
}

func FilterNameFn(name []string) func(Named) bool {
	return func(obj Named) bool {
		for _, v := range name {
			if strings.Contains(obj.Name(), v) {
				return true
			}
		}
		return false
	}
}

func FilterNamesFn(name []string) func(Names) bool {
	return func(obj Names) bool {
		for _, v := range name {
			for _, n := range obj.Names() {
				if strings.Contains(n, v) {
					return true
				}
			}
		}
		return false
	}
}

func FilterIDOrNameFn(id []string) func(IDOrNamed) bool {
	return func(obj IDOrNamed) bool {
		for _, v := range id {
			if strings.Contains(obj.Id(), v) || strings.Contains(obj.Name(), v) {
				return true
			}
		}
		return false
	}
}
