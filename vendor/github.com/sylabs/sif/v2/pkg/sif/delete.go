// Copyright (c) 2018-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2017, SingularityWare, LLC. All rights reserved.
// Copyright (c) 2017, Yannick Cote <yhcote@gmail.com> All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sif

import (
	"errors"
	"fmt"
	"io"
	"time"
)

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
