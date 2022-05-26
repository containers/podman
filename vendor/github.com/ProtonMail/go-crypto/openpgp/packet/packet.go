// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package packet implements parsing and serialization of OpenPGP packets, as
// specified in RFC 4880.
package packet // import "github.com/ProtonMail/go-crypto/openpgp/packet"

import (
	"bytes"
	"crypto/cipher"
	"crypto/rsa"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/algorithm"
)

// readFull is the same as io.ReadFull except that reading zero bytes returns
// ErrUnexpectedEOF rather than EOF.
func readFull(r io.Reader, buf []byte) (n int, err error) {
	n, err = io.ReadFull(r, buf)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}

// readLength reads an OpenPGP length from r. See RFC 4880, section 4.2.2.
func readLength(r io.Reader) (length int64, isPartial bool, err error) {
	var buf [4]byte
	_, err = readFull(r, buf[:1])
	if err != nil {
		return
	}
	switch {
	case buf[0] < 192:
		length = int64(buf[0])
	case buf[0] < 224:
		length = int64(buf[0]-192) << 8
		_, err = readFull(r, buf[0:1])
		if err != nil {
			return
		}
		length += int64(buf[0]) + 192
	case buf[0] < 255:
		length = int64(1) << (buf[0] & 0x1f)
		isPartial = true
	default:
		_, err = readFull(r, buf[0:4])
		if err != nil {
			return
		}
		length = int64(buf[0])<<24 |
			int64(buf[1])<<16 |
			int64(buf[2])<<8 |
			int64(buf[3])
	}
	return
}

// partialLengthReader wraps an io.Reader and handles OpenPGP partial lengths.
// The continuation lengths are parsed and removed from the stream and EOF is
// returned at the end of the packet. See RFC 4880, section 4.2.2.4.
type partialLengthReader struct {
	r         io.Reader
	remaining int64
	isPartial bool
}

func (r *partialLengthReader) Read(p []byte) (n int, err error) {
	for r.remaining == 0 {
		if !r.isPartial {
			return 0, io.EOF
		}
		r.remaining, r.isPartial, err = readLength(r.r)
		if err != nil {
			return 0, err
		}
	}

	toRead := int64(len(p))
	if toRead > r.remaining {
		toRead = r.remaining
	}

	n, err = r.r.Read(p[:int(toRead)])
	r.remaining -= int64(n)
	if n < int(toRead) && err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}

// partialLengthWriter writes a stream of data using OpenPGP partial lengths.
// See RFC 4880, section 4.2.2.4.
type partialLengthWriter struct {
	w          io.WriteCloser
	buf        bytes.Buffer
	lengthByte [1]byte
}

func (w *partialLengthWriter) Write(p []byte) (n int, err error) {
	bufLen := w.buf.Len()
	if bufLen > 512 {
		for power := uint(30); ; power-- {
			l := 1 << power
			if bufLen >= l {
				w.lengthByte[0] = 224 + uint8(power)
				_, err = w.w.Write(w.lengthByte[:])
				if err != nil {
					return
				}
				var m int
				m, err = w.w.Write(w.buf.Next(l))
				if err != nil {
					return
				}
				if m != l {
					return 0, io.ErrShortWrite
				}
				break
			}
		}
	}
	return w.buf.Write(p)
}

func (w *partialLengthWriter) Close() (err error) {
	len := w.buf.Len()
	err = serializeLength(w.w, len)
	if err != nil {
		return err
	}
	_, err = w.buf.WriteTo(w.w)
	if err != nil {
		return err
	}
	return w.w.Close()
}

// A spanReader is an io.LimitReader, but it returns ErrUnexpectedEOF if the
// underlying Reader returns EOF before the limit has been reached.
type spanReader struct {
	r io.Reader
	n int64
}

func (l *spanReader) Read(p []byte) (n int, err error) {
	if l.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.n {
		p = p[0:l.n]
	}
	n, err = l.r.Read(p)
	l.n -= int64(n)
	if l.n > 0 && err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}

// readHeader parses a packet header and returns an io.Reader which will return
// the contents of the packet. See RFC 4880, section 4.2.
func readHeader(r io.Reader) (tag packetType, length int64, contents io.Reader, err error) {
	var buf [4]byte
	_, err = io.ReadFull(r, buf[:1])
	if err != nil {
		return
	}
	if buf[0]&0x80 == 0 {
		err = errors.StructuralError("tag byte does not have MSB set")
		return
	}
	if buf[0]&0x40 == 0 {
		// Old format packet
		tag = packetType((buf[0] & 0x3f) >> 2)
		lengthType := buf[0] & 3
		if lengthType == 3 {
			length = -1
			contents = r
			return
		}
		lengthBytes := 1 << lengthType
		_, err = readFull(r, buf[0:lengthBytes])
		if err != nil {
			return
		}
		for i := 0; i < lengthBytes; i++ {
			length <<= 8
			length |= int64(buf[i])
		}
		contents = &spanReader{r, length}
		return
	}

	// New format packet
	tag = packetType(buf[0] & 0x3f)
	length, isPartial, err := readLength(r)
	if err != nil {
		return
	}
	if isPartial {
		contents = &partialLengthReader{
			remaining: length,
			isPartial: true,
			r:         r,
		}
		length = -1
	} else {
		contents = &spanReader{r, length}
	}
	return
}

// serializeHeader writes an OpenPGP packet header to w. See RFC 4880, section
// 4.2.
func serializeHeader(w io.Writer, ptype packetType, length int) (err error) {
	err = serializeType(w, ptype)
	if err != nil {
		return
	}
	return serializeLength(w, length)
}

// serializeType writes an OpenPGP packet type to w. See RFC 4880, section
// 4.2.
func serializeType(w io.Writer, ptype packetType) (err error) {
	var buf [1]byte
	buf[0] = 0x80 | 0x40 | byte(ptype)
	_, err = w.Write(buf[:])
	return
}

// serializeLength writes an OpenPGP packet length to w. See RFC 4880, section
// 4.2.2.
func serializeLength(w io.Writer, length int) (err error) {
	var buf [5]byte
	var n int

	if length < 192 {
		buf[0] = byte(length)
		n = 1
	} else if length < 8384 {
		length -= 192
		buf[0] = 192 + byte(length>>8)
		buf[1] = byte(length)
		n = 2
	} else {
		buf[0] = 255
		buf[1] = byte(length >> 24)
		buf[2] = byte(length >> 16)
		buf[3] = byte(length >> 8)
		buf[4] = byte(length)
		n = 5
	}

	_, err = w.Write(buf[:n])
	return
}

// serializeStreamHeader writes an OpenPGP packet header to w where the
// length of the packet is unknown. It returns a io.WriteCloser which can be
// used to write the contents of the packet. See RFC 4880, section 4.2.
func serializeStreamHeader(w io.WriteCloser, ptype packetType) (out io.WriteCloser, err error) {
	err = serializeType(w, ptype)
	if err != nil {
		return
	}
	out = &partialLengthWriter{w: w}
	return
}

// Packet represents an OpenPGP packet. Users are expected to try casting
// instances of this interface to specific packet types.
type Packet interface {
	parse(io.Reader) error
}

// consumeAll reads from the given Reader until error, returning the number of
// bytes read.
func consumeAll(r io.Reader) (n int64, err error) {
	var m int
	var buf [1024]byte

	for {
		m, err = r.Read(buf[:])
		n += int64(m)
		if err == io.EOF {
			err = nil
			return
		}
		if err != nil {
			return
		}
	}
}

// packetType represents the numeric ids of the different OpenPGP packet types. See
// http://www.iana.org/assignments/pgp-parameters/pgp-parameters.xhtml#pgp-parameters-2
type packetType uint8

const (
	packetTypeEncryptedKey              packetType = 1
	packetTypeSignature                 packetType = 2
	packetTypeSymmetricKeyEncrypted     packetType = 3
	packetTypeOnePassSignature          packetType = 4
	packetTypePrivateKey                packetType = 5
	packetTypePublicKey                 packetType = 6
	packetTypePrivateSubkey             packetType = 7
	packetTypeCompressed                packetType = 8
	packetTypeSymmetricallyEncrypted    packetType = 9
	packetTypeLiteralData               packetType = 11
	packetTypeUserId                    packetType = 13
	packetTypePublicSubkey              packetType = 14
	packetTypeUserAttribute             packetType = 17
	packetTypeSymmetricallyEncryptedMDC packetType = 18
	packetTypeAEADEncrypted             packetType = 20
)

// EncryptedDataPacket holds encrypted data. It is currently implemented by
// SymmetricallyEncrypted and AEADEncrypted.
type EncryptedDataPacket interface {
	Decrypt(CipherFunction, []byte) (io.ReadCloser, error)
}

// Read reads a single OpenPGP packet from the given io.Reader. If there is an
// error parsing a packet, the whole packet is consumed from the input.
func Read(r io.Reader) (p Packet, err error) {
	tag, _, contents, err := readHeader(r)
	if err != nil {
		return
	}

	switch tag {
	case packetTypeEncryptedKey:
		p = new(EncryptedKey)
	case packetTypeSignature:
		p = new(Signature)
	case packetTypeSymmetricKeyEncrypted:
		p = new(SymmetricKeyEncrypted)
	case packetTypeOnePassSignature:
		p = new(OnePassSignature)
	case packetTypePrivateKey, packetTypePrivateSubkey:
		pk := new(PrivateKey)
		if tag == packetTypePrivateSubkey {
			pk.IsSubkey = true
		}
		p = pk
	case packetTypePublicKey, packetTypePublicSubkey:
		isSubkey := tag == packetTypePublicSubkey
		p = &PublicKey{IsSubkey: isSubkey}
	case packetTypeCompressed:
		p = new(Compressed)
	case packetTypeSymmetricallyEncrypted:
		err = errors.UnsupportedError("Symmetrically encrypted packets without MDC are not supported")
	case packetTypeLiteralData:
		p = new(LiteralData)
	case packetTypeUserId:
		p = new(UserId)
	case packetTypeUserAttribute:
		p = new(UserAttribute)
	case packetTypeSymmetricallyEncryptedMDC:
		se := new(SymmetricallyEncrypted)
		se.MDC = true
		p = se
	case packetTypeAEADEncrypted:
		p = new(AEADEncrypted)
	default:
		err = errors.UnknownPacketTypeError(tag)
	}
	if p != nil {
		err = p.parse(contents)
	}
	if err != nil {
		consumeAll(contents)
	}
	return
}

// SignatureType represents the different semantic meanings of an OpenPGP
// signature. See RFC 4880, section 5.2.1.
type SignatureType uint8

const (
	SigTypeBinary            SignatureType = 0x00
	SigTypeText                            = 0x01
	SigTypeGenericCert                     = 0x10
	SigTypePersonaCert                     = 0x11
	SigTypeCasualCert                      = 0x12
	SigTypePositiveCert                    = 0x13
	SigTypeSubkeyBinding                   = 0x18
	SigTypePrimaryKeyBinding               = 0x19
	SigTypeDirectSignature                 = 0x1F
	SigTypeKeyRevocation                   = 0x20
	SigTypeSubkeyRevocation                = 0x28
	SigTypeCertificationRevocation         = 0x30
)

// PublicKeyAlgorithm represents the different public key system specified for
// OpenPGP. See
// http://www.iana.org/assignments/pgp-parameters/pgp-parameters.xhtml#pgp-parameters-12
type PublicKeyAlgorithm uint8

const (
	PubKeyAlgoRSA     PublicKeyAlgorithm = 1
	PubKeyAlgoElGamal PublicKeyAlgorithm = 16
	PubKeyAlgoDSA     PublicKeyAlgorithm = 17
	// RFC 6637, Section 5.
	PubKeyAlgoECDH  PublicKeyAlgorithm = 18
	PubKeyAlgoECDSA PublicKeyAlgorithm = 19
	// https://www.ietf.org/archive/id/draft-koch-eddsa-for-openpgp-04.txt
	PubKeyAlgoEdDSA PublicKeyAlgorithm = 22

	// Deprecated in RFC 4880, Section 13.5. Use key flags instead.
	PubKeyAlgoRSAEncryptOnly PublicKeyAlgorithm = 2
	PubKeyAlgoRSASignOnly    PublicKeyAlgorithm = 3
)

// CanEncrypt returns true if it's possible to encrypt a message to a public
// key of the given type.
func (pka PublicKeyAlgorithm) CanEncrypt() bool {
	switch pka {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly, PubKeyAlgoElGamal, PubKeyAlgoECDH:
		return true
	}
	return false
}

// CanSign returns true if it's possible for a public key of the given type to
// sign a message.
func (pka PublicKeyAlgorithm) CanSign() bool {
	switch pka {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly, PubKeyAlgoDSA, PubKeyAlgoECDSA, PubKeyAlgoEdDSA:
		return true
	}
	return false
}

// CipherFunction represents the different block ciphers specified for OpenPGP. See
// http://www.iana.org/assignments/pgp-parameters/pgp-parameters.xhtml#pgp-parameters-13
type CipherFunction algorithm.CipherFunction

const (
	Cipher3DES   CipherFunction = 2
	CipherCAST5  CipherFunction = 3
	CipherAES128 CipherFunction = 7
	CipherAES192 CipherFunction = 8
	CipherAES256 CipherFunction = 9
)

// KeySize returns the key size, in bytes, of cipher.
func (cipher CipherFunction) KeySize() int {
	return algorithm.CipherFunction(cipher).KeySize()
}

// blockSize returns the block size, in bytes, of cipher.
func (cipher CipherFunction) blockSize() int {
	return algorithm.CipherFunction(cipher).BlockSize()
}

// new returns a fresh instance of the given cipher.
func (cipher CipherFunction) new(key []byte) (block cipher.Block) {
	return algorithm.CipherFunction(cipher).New(key)
}

// padToKeySize left-pads a MPI with zeroes to match the length of the
// specified RSA public.
func padToKeySize(pub *rsa.PublicKey, b []byte) []byte {
	k := (pub.N.BitLen() + 7) / 8
	if len(b) >= k {
		return b
	}
	bb := make([]byte, k)
	copy(bb[len(bb)-len(b):], b)
	return bb
}

// CompressionAlgo Represents the different compression algorithms
// supported by OpenPGP (except for BZIP2, which is not currently
// supported). See Section 9.3 of RFC 4880.
type CompressionAlgo uint8

const (
	CompressionNone CompressionAlgo = 0
	CompressionZIP  CompressionAlgo = 1
	CompressionZLIB CompressionAlgo = 2
)

// AEADMode represents the different Authenticated Encryption with Associated
// Data specified for OpenPGP.
type AEADMode algorithm.AEADMode

const (
	AEADModeEAX             AEADMode = 1
	AEADModeOCB             AEADMode = 2
	AEADModeExperimentalGCM AEADMode = 100
)

func (mode AEADMode) NonceLength() int {
	return algorithm.AEADMode(mode).NonceLength()
}

func (mode AEADMode) TagLength() int {
	return algorithm.AEADMode(mode).TagLength()
}

// new returns a fresh instance of the given mode.
func (mode AEADMode) new(block cipher.Block) cipher.AEAD {
	return algorithm.AEADMode(mode).New(block)
}

// ReasonForRevocation represents a revocation reason code as per RFC4880
// section 5.2.3.23.
type ReasonForRevocation uint8

const (
	NoReason       ReasonForRevocation = 0
	KeySuperseded  ReasonForRevocation = 1
	KeyCompromised ReasonForRevocation = 2
	KeyRetired     ReasonForRevocation = 3
)
