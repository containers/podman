//
// Copyright (c) SAS Institute Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// PKCS#9 is a specification for trusted timestamping. Timestamping services
// create a timestamp token which includes a known-good timestamp with a
// signature over it. The token can be attached to a document to prove that it
// existed at the indicated time. When attached to a PKCS#7 signedData
// structure, the timestamp proves that the primary signature was created
// during the valid lifespan of the signing certificate, allowing it to be
// validated after the certificates have expired.
//
// See RFC 3161
package pkcs9

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"time"

	"github.com/sassoftware/relic/lib/pkcs7"
)

const (
	StatusGranted = iota
	StatusGrantedWithMods
	StatusRejection
	StatusWaiting
	StatusRevocationWarning
	StatusRevocationNotification

	FailureBadAlg              = 0
	FailureBadRequest          = 2
	FailureBadDataFormat       = 5
	FailureTimeNotAvailable    = 14
	FailureUnacceptedPolicy    = 15
	FailureUnacceptedExtension = 16
	FailureAddInfoNotAvailable = 17
	SystemFailure              = 25
)

var (
	OidKeyPurposeTimeStamping  = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 8}
	OidTSTInfo                 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4}
	OidAttributeTimeStampToken = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}
	OidAttributeCounterSign    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 6}

	OidSpcTimeStampRequest = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 3, 2, 1}
	// undocumented(?) alternative to OidAttributeTimeStampToken found in Authenticode signatures
	OidSpcTimeStampToken = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 3, 3, 1}
)

type TimeStampReq struct {
	Version        int
	MessageImprint MessageImprint
	ReqPolicy      asn1.ObjectIdentifier `asn1:"optional"`
	Nonce          *big.Int              `asn1:"optional"`
	CertReq        bool                  `asn1:"default:false"`
	Extensions     []pkix.Extension      `asn1:"optional,implicit,tag:0"`
}

type MessageImprint struct {
	HashAlgorithm pkix.AlgorithmIdentifier
	HashedMessage []byte
}

type TimeStampResp struct {
	Status         PKIStatusInfo
	TimeStampToken pkcs7.ContentInfoSignedData `asn1:"optional"`
}

type PKIStatusInfo struct {
	Status       int
	StatusString []string       `asn1:"optional"`
	FailInfo     asn1.BitString `asn1:"optional"`
}

type TSTInfo struct {
	Version        int
	Policy         asn1.ObjectIdentifier
	MessageImprint MessageImprint
	SerialNumber   *big.Int
	GenTime        asn1.RawValue
	Accuracy       Accuracy         `asn1:"optional"`
	Ordering       bool             `asn1:"optional,default:false"`
	Nonce          *big.Int         `asn1:"optional"`
	TSA            GeneralName      `asn1:"optional,implicit,tag:0"`
	Extensions     []pkix.Extension `asn1:"optional,implicit,tag:1"`
}

func (i *TSTInfo) SigningTime() (time.Time, error) {
	return pkcs7.ParseTime(i.GenTime)
}

type Accuracy struct {
	Seconds int `asn1:"optional"`
	Millis  int `asn1:"optional,tag:0"`
	Micros  int `asn1:"optional,tag:1"`
}

type GeneralName struct {
	// See RFC 3280
	Value asn1.RawValue
}
