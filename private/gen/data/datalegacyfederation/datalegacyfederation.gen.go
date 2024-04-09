// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by buf-legacyfederation-go-data. DO NOT EDIT.

package datalegacyfederation

import (
	"encoding/hex"
	"strings"

	"github.com/bufbuild/buf/private/pkg/shake256"
)

var (
	// hostnameHexEncodedDigests are the shake256 digests of the hostnames that are allowed to use legacy federation.
	hostnameHexEncodedDigests = map[string]struct{}{
		"a9dcb9abaac3a9c1a21a5e2e3aa709133c26136cdeda5e112d8fd6f60629671ff900c23c26d24c4558f265ba4d520daaffc2f69e7f9523c4f26bc2b7d98be607": {},
		"ae726fef74c38124ebb06526dbc533d9fa4b975cdbefb697be18e02e94e57b0d44680a73e7a7f1378e83fb71efcccfe950992d641f7b2286430713ed817d4fdd": {},
	}
)

// Exists returns true if the hostname is allowed to use legacy federation.
func Exists(hostname string) (bool, error) {
	if hostname == "" {
		return false, nil
	}
	digest, err := shake256.NewDigestForContent(strings.NewReader(hostname))
	if err != nil {
		return false, err
	}
	hexEncodedDigest := hex.EncodeToString(digest.Value())
	_, ok := hostnameHexEncodedDigests[hexEncodedDigest]
	return ok, nil
}