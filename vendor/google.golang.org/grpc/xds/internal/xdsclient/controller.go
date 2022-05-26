/*
 *
 * Copyright 2021 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xdsclient

import (
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/xds/internal/xdsclient/bootstrap"
	"google.golang.org/grpc/xds/internal/xdsclient/controller"
	"google.golang.org/grpc/xds/internal/xdsclient/load"
	"google.golang.org/grpc/xds/internal/xdsclient/pubsub"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

type controllerInterface interface {
	AddWatch(resourceType xdsresource.ResourceType, resourceName string)
	RemoveWatch(resourceType xdsresource.ResourceType, resourceName string)
	ReportLoad(server string) (*load.Store, func())
	Close()
}

var newController = func(config *bootstrap.ServerConfig, pubsub *pubsub.Pubsub, validator xdsresource.UpdateValidatorFunc, logger *grpclog.PrefixLogger) (controllerInterface, error) {
	return controller.New(config, pubsub, validator, logger)
}
