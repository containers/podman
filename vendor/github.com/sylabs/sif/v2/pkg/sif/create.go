// Copyright (c) 2018-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2017, SingularityWare, LLC. All rights reserved.
// Copyright (c) 2017, Yannick Cote <yhcote@gmail.com> All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sif

import (
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
)

// nextAligned finds the next offset that satisfies alignment.
func nextAligned(offset int64, alignment int) int64 {
	align64 := uint64(alignment)
	offset64 := uint64(offset)

	if align64 != 0 && offset64%align64 != 0 {
		offset64 = (offset64 & ^(align64 - 1)) + align64
	}

	return int64(offset64)
}

// writeDataObjectAt writes the data object described by di to ws, using time t, recording details
// in d. The object is written at the first position that satisfies the alignment requirements
// described by di following offsetUnaligned.
func writeDataObjectAt(ws io.WriteSeeker, offsetUnaligned int64, di DescriptorInput, t time.Time, d *rawDescriptor) error { //nolint:lll
	offset, err := ws.Seek(nextAligned(offsetUnaligned, di.opts.alignment), io.SeekStart)
	if err != nil {
		return err
	}

	n, err := io.Copy(ws, di.r)
	if err != nil {
		return err
	}

	if err := di.fillDescriptor(t, d); err != nil {
		return err
	}
	d.Used = true
	d.Offset = offset
	d.Size = n
	d.SizeWithPadding = offset - offsetUnaligned + n

	return nil
}

var (
	errInsufficientCapacity = errors.New("insufficient descriptor capacity to add data object(s) to image")
	errPrimaryPartition     = errors.New("image already contains a primary partition")
)

// writeDataObject writes the data object described by di to f, using time t, recording details in
// the descriptor at index i.
func (f *FileImage) writeDataObject(i int, di DescriptorInput, t time.Time) error {
	if i >= len(f.rds) {
		return errInsufficientCapacity
	}

	// If this is a primary partition, verify there isn't another primary partition, and update the
	// architecture in the global header.
	if p, ok := di.opts.md.(partition); ok && p.Parttype == PartPrimSys {
		if ds, err := f.GetDescriptors(WithPartitionType(PartPrimSys)); err == nil && len(ds) > 0 {
			return errPrimaryPartition
		}

		f.h.Arch = p.Arch
	}

	d := &f.rds[i]
	d.ID = uint32(i) + 1

	if err := writeDataObjectAt(f.rw, f.h.DataOffset+f.h.DataSize, di, t, d); err != nil {
		return err
	}

	// Update minimum object ID map.
	if minID, ok := f.minIDs[d.GroupID]; !ok || d.ID < minID {
		f.minIDs[d.GroupID] = d.ID
	}

	f.h.DescriptorsFree--
	f.h.DataSize += d.SizeWithPadding

	return nil
}

// writeDescriptors writes the descriptors in f to backing storage.
func (f *FileImage) writeDescriptors() error {
	if _, err := f.rw.Seek(f.h.DescriptorsOffset, io.SeekStart); err != nil {
		return err
	}

	return binary.Write(f.rw, binary.LittleEndian, f.rds)
}

// writeHeader writes the global header in f to backing storage.
func (f *FileImage) writeHeader() error {
	if _, err := f.rw.Seek(0, io.SeekStart); err != nil {
		return err
	}

	return binary.Write(f.rw, binary.LittleEndian, f.h)
}

// createOpts accumulates container creation options.
type createOpts struct {
	launchScript       [hdrLaunchLen]byte
	id                 uuid.UUID
	descriptorsOffset  int64
	descriptorCapacity int64
	dis                []DescriptorInput
	t                  time.Time
	closeOnUnload      bool
}

// CreateOpt are used to specify container creation options.
type CreateOpt func(*createOpts) error

var errLaunchScriptLen = errors.New("launch script too large")

// OptCreateWithLaunchScript specifies s as the launch script.
func OptCreateWithLaunchScript(s string) CreateOpt {
	return func(co *createOpts) error {
		b := []byte(s)

		if len(b) >= len(co.launchScript) {
			return errLaunchScriptLen
		}

		copy(co.launchScript[:], b)

		return nil
	}
}

// OptCreateDeterministic sets header/descriptor fields to values that support deterministic
// creation of images.
func OptCreateDeterministic() CreateOpt {
	return func(co *createOpts) error {
		co.id = uuid.Nil
		co.t = time.Time{}
		return nil
	}
}

// OptCreateWithID specifies id as the unique ID.
func OptCreateWithID(id string) CreateOpt {
	return func(co *createOpts) error {
		id, err := uuid.Parse(id)
		co.id = id
		return err
	}
}

// OptCreateWithDescriptorCapacity specifies that the created image should have the capacity for a
// maximum of n descriptors.
func OptCreateWithDescriptorCapacity(n int64) CreateOpt {
	return func(co *createOpts) error {
		co.descriptorCapacity = n
		return nil
	}
}

// OptCreateWithDescriptors appends dis to the list of descriptors.
func OptCreateWithDescriptors(dis ...DescriptorInput) CreateOpt {
	return func(co *createOpts) error {
		co.dis = append(co.dis, dis...)
		return nil
	}
}

// OptCreateWithTime specifies t as the image creation time.
func OptCreateWithTime(t time.Time) CreateOpt {
	return func(co *createOpts) error {
		co.t = t
		return nil
	}
}

// OptCreateWithCloseOnUnload specifies whether the ReadWriter should be closed by UnloadContainer.
// By default, the ReadWriter will be closed if it implements the io.Closer interface.
func OptCreateWithCloseOnUnload(b bool) CreateOpt {
	return func(co *createOpts) error {
		co.closeOnUnload = b
		return nil
	}
}

// createContainer creates a new SIF container file in rw, according to opts.
func createContainer(rw ReadWriter, co createOpts) (*FileImage, error) {
	rds := make([]rawDescriptor, co.descriptorCapacity)
	rdsSize := int64(binary.Size(rds))

	h := header{
		LaunchScript:      co.launchScript,
		Magic:             hdrMagic,
		Version:           CurrentVersion.bytes(),
		Arch:              hdrArchUnknown,
		ID:                co.id,
		CreatedAt:         co.t.Unix(),
		ModifiedAt:        co.t.Unix(),
		DescriptorsFree:   co.descriptorCapacity,
		DescriptorsTotal:  co.descriptorCapacity,
		DescriptorsOffset: co.descriptorsOffset,
		DescriptorsSize:   rdsSize,
		DataOffset:        co.descriptorsOffset + rdsSize,
	}

	f := &FileImage{
		rw:     rw,
		h:      h,
		rds:    rds,
		minIDs: make(map[uint32]uint32),
	}

	for i, di := range co.dis {
		if err := f.writeDataObject(i, di, co.t); err != nil {
			return nil, err
		}
	}

	if err := f.writeDescriptors(); err != nil {
		return nil, err
	}

	if err := f.writeHeader(); err != nil {
		return nil, err
	}

	return f, nil
}

// CreateContainer creates a new SIF container in rw, according to opts. One or more data objects
// can optionally be specified using OptCreateWithDescriptors.
//
// On success, a FileImage is returned. The caller must call UnloadContainer to ensure resources
// are released. By default, UnloadContainer will close rw if it implements the io.Closer
// interface. To change this behavior, consider using OptCreateWithCloseOnUnload.
//
// By default, the image ID is set to a randomly generated value. To override this, consider using
// OptCreateDeterministic or OptCreateWithID.
//
// By default, the image creation time is set to the current time. To override this, consider using
// OptCreateDeterministic or OptCreateWithTime.
//
// By default, the image will support a maximum of 48 descriptors. To change this, consider using
// OptCreateWithDescriptorCapacity.
//
// A launch script can optionally be set using OptCreateWithLaunchScript.
func CreateContainer(rw ReadWriter, opts ...CreateOpt) (*FileImage, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	co := createOpts{
		id:                 id,
		descriptorsOffset:  4096,
		descriptorCapacity: 48,
		t:                  time.Now(),
		closeOnUnload:      true,
	}

	for _, opt := range opts {
		if err := opt(&co); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}

	f, err := createContainer(rw, co)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	f.closeOnUnload = co.closeOnUnload
	return f, nil
}

// CreateContainerAtPath creates a new SIF container file at path, according to opts. One or more
// data objects can optionally be specified using OptCreateWithDescriptors.
//
// On success, a FileImage is returned. The caller must call UnloadContainer to ensure resources
// are released.
//
// By default, the image ID is set to a randomly generated value. To override this, consider using
// OptCreateDeterministic or OptCreateWithID.
//
// By default, the image creation time is set to the current time. To override this, consider using
// OptCreateDeterministic or OptCreateWithTime.
//
// By default, the image will support a maximum of 48 descriptors. To change this, consider using
// OptCreateWithDescriptorCapacity.
//
// A launch script can optionally be set using OptCreateWithLaunchScript.
func CreateContainerAtPath(path string, opts ...CreateOpt) (*FileImage, error) {
	fp, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	f, err := CreateContainer(fp, opts...)
	if err != nil {
		fp.Close()
		os.Remove(fp.Name())

		return nil, err
	}

	f.closeOnUnload = true
	return f, nil
}

// addOpts accumulates object add options.
type addOpts struct {
	t time.Time
}

// AddOpt are used to specify object add options.
type AddOpt func(*addOpts) error

// OptAddDeterministic sets header/descriptor fields to values that support deterministic
// modification of images.
func OptAddDeterministic() AddOpt {
	return func(ao *addOpts) error {
		ao.t = time.Time{}
		return nil
	}
}

// OptAddWithTime specifies t as the image modification time.
func OptAddWithTime(t time.Time) AddOpt {
	return func(ao *addOpts) error {
		ao.t = t
		return nil
	}
}

// AddObject adds a new data object and its descriptor into the specified SIF file.
//
// By default, the image modification time is set to the current time for non-deterministic images,
// and unset otherwise. To override this, consider using OptAddDeterministic or OptAddWithTime.
func (f *FileImage) AddObject(di DescriptorInput, opts ...AddOpt) error {
	ao := addOpts{}

	if !f.isDeterministic() {
		ao.t = time.Now()
	}

	for _, opt := range opts {
		if err := opt(&ao); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	// Find an unused descriptor.
	i := 0
	for _, rd := range f.rds {
		if !rd.Used {
			break
		}
		i++
	}

	if err := f.writeDataObject(i, di, ao.t); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := f.writeDescriptors(); err != nil {
		return fmt.Errorf("%w", err)
	}

	f.h.ModifiedAt = ao.t.Unix()

	if err := f.writeHeader(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// isLast return true if the data object associated with d is the last in f.
func (f *FileImage) isLast(d *rawDescriptor) bool {
	isLast := true

	end := d.Offset + d.Size
	f.WithDescriptors(func(d Descriptor) bool {
		isLast = d.Offset()+d.Size() <= end
		return !isLast
	})

	return isLast
}

// zeroReader is an io.Reader that returns a stream of zero-bytes.
type zeroReader struct{}

func (zeroReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 0
	}
	return len(b), nil
}

// zero overwrites the data object described by d with a stream of zero bytes.
func (f *FileImage) zero(d *rawDescriptor) error {
	if _, err := f.rw.Seek(d.Offset, io.SeekStart); err != nil {
		return err
	}

	_, err := io.CopyN(f.rw, zeroReader{}, d.Size)
	return err
}

// truncateAt truncates f at the start of the padded data object described by d.
func (f *FileImage) truncateAt(d *rawDescriptor) error {
	start := d.Offset + d.Size - d.SizeWithPadding

	return f.rw.Truncate(start)
}

// deleteOpts accumulates object deletion options.
type deleteOpts struct {
	zero    bool
	compact bool
	t       time.Time
}

// DeleteOpt are used to specify object deletion options.
type DeleteOpt func(*deleteOpts) error

// OptDeleteZero specifies whether the deleted object should be zeroed.
func OptDeleteZero(b bool) DeleteOpt {
	return func(do *deleteOpts) error {
		do.zero = b
		return nil
	}
}

// OptDeleteCompact specifies whether the image should be compacted following object deletion.
func OptDeleteCompact(b bool) DeleteOpt {
	return func(do *deleteOpts) error {
		do.compact = b
		return nil
	}
}

// OptDeleteDeterministic sets header/descriptor fields to values that support deterministic
// modification of images.
func OptDeleteDeterministic() DeleteOpt {
	return func(do *deleteOpts) error {
		do.t = time.Time{}
		return nil
	}
}

// OptDeleteWithTime specifies t as the image modification time.
func OptDeleteWithTime(t time.Time) DeleteOpt {
	return func(do *deleteOpts) error {
		do.t = t
		return nil
	}
}

var errCompactNotImplemented = errors.New("compact not implemented for non-last object")

// DeleteObject deletes the data object with id, according to opts.
//
// To zero the data region of the deleted object, use OptDeleteZero. To compact the file following
// object deletion, use OptDeleteCompact.
//
// By default, the image modification time is set to the current time for non-deterministic images,
// and unset otherwise. To override this, consider using OptDeleteDeterministic or
// OptDeleteWithTime.
func (f *FileImage) DeleteObject(id uint32, opts ...DeleteOpt) error {
	do := deleteOpts{}

	if !f.isDeterministic() {
		do.t = time.Now()
	}

	for _, opt := range opts {
		if err := opt(&do); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	d, err := f.getDescriptor(WithID(id))
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if do.compact && !f.isLast(d) {
		return fmt.Errorf("%w", errCompactNotImplemented)
	}

	if do.zero {
		if err := f.zero(d); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if do.compact {
		if err := f.truncateAt(d); err != nil {
			return fmt.Errorf("%w", err)
		}

		f.h.DataSize -= d.SizeWithPadding
	}

	f.h.DescriptorsFree++
	f.h.ModifiedAt = do.t.Unix()

	// If we remove the primary partition, set the global header Arch field to HdrArchUnknown
	// to indicate that the SIF file doesn't include a primary partition and no dependency
	// on any architecture exists.
	if d.isPartitionOfType(PartPrimSys) {
		f.h.Arch = hdrArchUnknown
	}

	// Reset rawDescripter with empty struct
	*d = rawDescriptor{}

	if err := f.writeDescriptors(); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := f.writeHeader(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// setOpts accumulates object set options.
type setOpts struct {
	t time.Time
}

// SetOpt are used to specify object set options.
type SetOpt func(*setOpts) error

// OptSetDeterministic sets header/descriptor fields to values that support deterministic
// modification of images.
func OptSetDeterministic() SetOpt {
	return func(so *setOpts) error {
		so.t = time.Time{}
		return nil
	}
}

// OptSetWithTime specifies t as the image/object modification time.
func OptSetWithTime(t time.Time) SetOpt {
	return func(so *setOpts) error {
		so.t = t
		return nil
	}
}

var (
	errNotPartition = errors.New("data object not a partition")
	errNotSystem    = errors.New("data object not a system partition")
)

// SetPrimPart sets the specified system partition to be the primary one.
//
// By default, the image/object modification times are set to the current time for
// non-deterministic images, and unset otherwise. To override this, consider using
// OptSetDeterministic or OptSetWithTime.
func (f *FileImage) SetPrimPart(id uint32, opts ...SetOpt) error {
	so := setOpts{}

	if !f.isDeterministic() {
		so.t = time.Now()
	}

	for _, opt := range opts {
		if err := opt(&so); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	descr, err := f.getDescriptor(WithID(id))
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if descr.DataType != DataPartition {
		return fmt.Errorf("%w", errNotPartition)
	}

	var p partition
	if err := descr.getExtra(binaryUnmarshaler{&p}); err != nil {
		return fmt.Errorf("%w", err)
	}

	// if already primary system partition, nothing to do
	if p.Parttype == PartPrimSys {
		return nil
	}

	if p.Parttype != PartSystem {
		return fmt.Errorf("%w", errNotSystem)
	}

	// If there is currently a primary system partition, update it.
	if d, err := f.getDescriptor(WithPartitionType(PartPrimSys)); err == nil {
		var p partition
		if err := d.getExtra(binaryUnmarshaler{&p}); err != nil {
			return fmt.Errorf("%w", err)
		}

		p.Parttype = PartSystem

		if err := d.setExtra(p); err != nil {
			return fmt.Errorf("%w", err)
		}

		d.ModifiedAt = so.t.Unix()
	} else if !errors.Is(err, ErrObjectNotFound) {
		return fmt.Errorf("%w", err)
	}

	// Update the descriptor of the new primary system partition.
	p.Parttype = PartPrimSys

	if err := descr.setExtra(p); err != nil {
		return fmt.Errorf("%w", err)
	}

	descr.ModifiedAt = so.t.Unix()

	if err := f.writeDescriptors(); err != nil {
		return fmt.Errorf("%w", err)
	}

	f.h.Arch = p.Arch
	f.h.ModifiedAt = so.t.Unix()

	if err := f.writeHeader(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// SetMetadata sets the metadata of the data object with id to md, according to opts.
//
// By default, the image/object modification times are set to the current time for
// non-deterministic images, and unset otherwise. To override this, consider using
// OptSetDeterministic or OptSetWithTime.
func (f *FileImage) SetMetadata(id uint32, md encoding.BinaryMarshaler, opts ...SetOpt) error {
	so := setOpts{}

	if !f.isDeterministic() {
		so.t = time.Now()
	}

	for _, opt := range opts {
		if err := opt(&so); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	rd, err := f.getDescriptor(WithID(id))
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := rd.setExtra(md); err != nil {
		return fmt.Errorf("%w", err)
	}

	rd.ModifiedAt = so.t.Unix()

	if err := f.writeDescriptors(); err != nil {
		return fmt.Errorf("%w", err)
	}

	f.h.ModifiedAt = so.t.Unix()

	if err := f.writeHeader(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
