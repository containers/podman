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
	"crypto"
	"crypto/hmac"
	"encoding/asn1"
	"errors"
	"fmt"
	"net/http"

	"github.com/sassoftware/relic/lib/pkcs7"
	"github.com/sassoftware/relic/lib/x509tools"
)

// RFC 3161 timestamping

// Create a HTTP request to request a token from the given URL
func NewRequest(url string, hash crypto.Hash, hashValue []byte) (msg *TimeStampReq, req *http.Request, err error) {
	alg, ok := x509tools.PkixDigestAlgorithm(hash)
	if !ok {
		return nil, nil, errors.New("unknown digest algorithm")
	}
	msg = &TimeStampReq{
		Version: 1,
		MessageImprint: MessageImprint{
			HashAlgorithm: alg,
			HashedMessage: hashValue,
		},
		Nonce:   x509tools.MakeSerial(),
		CertReq: true,
	}
	reqbytes, err := asn1.Marshal(*msg)
	if err != nil {
		return
	}
	req, err = http.NewRequest("POST", url, bytes.NewReader(reqbytes))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/timestamp-query")
	return
}

// Parse a timestamp token from a HTTP response, sanity checking it against the original request nonce
func (req *TimeStampReq) ParseResponse(body []byte) (*pkcs7.ContentInfoSignedData, error) {
	respmsg := new(TimeStampResp)
	if rest, err := asn1.Unmarshal(body, respmsg); err != nil {
		return nil, fmt.Errorf("pkcs9: unmarshalling response: %w", err)
	} else if len(rest) != 0 {
		return nil, errors.New("pkcs9: trailing bytes in response")
	} else if respmsg.Status.Status > StatusGrantedWithMods {
		return nil, fmt.Errorf("pkcs9: request denied: status=%d failureInfo=%x", respmsg.Status.Status, respmsg.Status.FailInfo.Bytes)
	}
	if err := req.SanityCheckToken(&respmsg.TimeStampToken); err != nil {
		return nil, fmt.Errorf("pkcs9: token sanity check failed: %w", err)
	}
	return &respmsg.TimeStampToken, nil
}

// Sanity check a timestamp token against the nonce in the original request
func (req *TimeStampReq) SanityCheckToken(psd *pkcs7.ContentInfoSignedData) error {
	if _, err := psd.Content.Verify(nil, false); err != nil {
		return err
	}
	info, err := unpackTokenInfo(psd)
	if err != nil {
		return err
	}
	if req.Nonce.Cmp(info.Nonce) != 0 {
		return errors.New("request nonce mismatch")
	}
	if !hmac.Equal(info.MessageImprint.HashedMessage, req.MessageImprint.HashedMessage) {
		return errors.New("message imprint mismatch")
	}
	return nil
}

// Unpack TSTInfo from a timestamp token
func unpackTokenInfo(psd *pkcs7.ContentInfoSignedData) (*TSTInfo, error) {
	infobytes, err := psd.Content.ContentInfo.Bytes()
	if err != nil {
		return nil, fmt.Errorf("unpack TSTInfo: %w", err)
	} else if infobytes[0] == 0x04 {
		// unwrap dummy OCTET STRING
		_, err = asn1.Unmarshal(infobytes, &infobytes)
		if err != nil {
			return nil, fmt.Errorf("unpack TSTInfo: %w", err)
		}
	}
	info := new(TSTInfo)
	if _, err := asn1.Unmarshal(infobytes, info); err != nil {
		return nil, fmt.Errorf("unpack TSTInfo: %w", err)
	}
	return info, nil
}
