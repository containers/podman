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

package pkcs9

import (
	"bytes"
	"encoding/asn1"
	"encoding/base64"
	"net/http"

	"github.com/sassoftware/relic/lib/pkcs7"
)

// Microsoft non-RFC-3161 timestamping
// https://msdn.microsoft.com/en-us/library/windows/desktop/bb931395(v=vs.85).aspx

type MicrosoftTimeStampRequest struct {
	CounterSignatureType asn1.ObjectIdentifier
	Attributes           struct{} `asn1:"optional"`
	Content              struct {
		ContentType asn1.ObjectIdentifier
		Content     []byte `asn1:"explicit,tag:0"`
	}
}

func NewLegacyRequest(url string, encryptedDigest []byte) (*http.Request, error) {
	var msg MicrosoftTimeStampRequest
	msg.CounterSignatureType = OidSpcTimeStampRequest
	msg.Content.ContentType = pkcs7.OidData
	msg.Content.Content = encryptedDigest
	blob, err := asn1.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	return req, nil
}

func ParseLegacyResponse(body []byte) (*pkcs7.ContentInfoSignedData, error) {
	rblob, err := base64.StdEncoding.DecodeString(string(bytes.TrimRight(body, "\x00")))
	if err != nil {
		return nil, err
	}
	psd := new(pkcs7.ContentInfoSignedData)
	if _, err := asn1.Unmarshal(rblob, psd); err != nil {
		return nil, err
	}
	return psd, nil
}
