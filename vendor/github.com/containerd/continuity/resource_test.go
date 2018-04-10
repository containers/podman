package continuity

type resourceUpdate struct {
	Original Resource
	Updated  Resource
}

type resourceListDifference struct {
	Additions []Resource
	Deletions []Resource
	Updates   []resourceUpdate
}

func (l resourceListDifference) HasDiff() bool {
	return len(l.Additions) > 0 || len(l.Deletions) > 0 || len(l.Updates) > 0
}

// diffManifest compares two resource lists and returns the list
// of adds updates and deletes, resource lists are not reordered
// before doing difference.
func diffResourceList(r1, r2 []Resource) resourceListDifference {
	i1 := 0
	i2 := 0
	var d resourceListDifference

	for i1 < len(r1) && i2 < len(r2) {
		p1 := r1[i1].Path()
		p2 := r2[i2].Path()
		switch {
		case p1 < p2:
			d.Deletions = append(d.Deletions, r1[i1])
			i1++
		case p1 == p2:
			if !compareResource(r1[i1], r2[i2]) {
				d.Updates = append(d.Updates, resourceUpdate{
					Original: r1[i1],
					Updated:  r2[i2],
				})
			}
			i1++
			i2++
		case p1 > p2:
			d.Additions = append(d.Additions, r2[i2])
			i2++
		}
	}

	for i1 < len(r1) {
		d.Deletions = append(d.Deletions, r1[i1])
		i1++

	}
	for i2 < len(r2) {
		d.Additions = append(d.Additions, r2[i2])
		i2++
	}

	return d
}

func compareResource(r1, r2 Resource) bool {
	if r1.Path() != r2.Path() {
		return false
	}
	if r1.Mode() != r2.Mode() {
		return false
	}
	if r1.UID() != r2.UID() {
		return false
	}
	if r1.GID() != r2.GID() {
		return false
	}

	// TODO(dmcgowan): Check if is XAttrer

	switch t1 := r1.(type) {
	case RegularFile:
		t2, ok := r2.(RegularFile)
		if !ok {
			return false
		}
		return compareRegularFile(t1, t2)
	case Directory:
		t2, ok := r2.(Directory)
		if !ok {
			return false
		}
		return compareDirectory(t1, t2)
	case SymLink:
		t2, ok := r2.(SymLink)
		if !ok {
			return false
		}
		return compareSymLink(t1, t2)
	case NamedPipe:
		t2, ok := r2.(NamedPipe)
		if !ok {
			return false
		}
		return compareNamedPipe(t1, t2)
	case Device:
		t2, ok := r2.(Device)
		if !ok {
			return false
		}
		return compareDevice(t1, t2)
	default:
		// TODO(dmcgowan): Should this panic?
		return r1 == r2
	}
}

func compareRegularFile(r1, r2 RegularFile) bool {
	if r1.Size() != r2.Size() {
		return false
	}
	p1 := r1.Paths()
	p2 := r2.Paths()
	if len(p1) != len(p2) {
		return false
	}
	for i := range p1 {
		if p1[i] != p2[i] {
			return false
		}
	}
	d1 := r1.Digests()
	d2 := r2.Digests()
	if len(d1) != len(d2) {
		return false
	}
	for i := range d1 {
		if d1[i] != d2[i] {
			return false
		}
	}

	return true
}

func compareSymLink(r1, r2 SymLink) bool {
	return r1.Target() == r2.Target()
}

func compareDirectory(r1, r2 Directory) bool {
	return true
}

func compareNamedPipe(r1, r2 NamedPipe) bool {
	return true
}

func compareDevice(r1, r2 Device) bool {
	return r1.Major() == r2.Major() && r1.Minor() == r2.Minor()
}
