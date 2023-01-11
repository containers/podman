// Copyright 2016 Google LLC. All Rights Reserved.
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

package trillian

//go:generate protoc -I=. -I=third_party/googleapis --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. --go-grpc_opt=require_unimplemented_servers=false trillian_log_api.proto trillian_admin_api.proto trillian.proto --doc_out=markdown,api.md:./docs/
//go:generate protoc -I=. --go_out=paths=source_relative:. crypto/keyspb/keyspb.proto

//go:generate mockgen -package tmock -destination testonly/tmock/mock_log_server.go  github.com/google/trillian TrillianLogServer
//go:generate mockgen -package tmock -destination testonly/tmock/mock_admin_server.go github.com/google/trillian TrillianAdminServer
