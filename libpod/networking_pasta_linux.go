// SPDX-License-Identifier: Apache-2.0
//
// networking_pasta_linux.go - Start pasta(1) for user-mode connectivity
//
// Copyright (c) 2022 Red Hat GmbH
// Author: Stefano Brivio <sbrivio@redhat.com>

package libpod

import "github.com/containers/common/libnetwork/pasta"

func (r *Runtime) setupPasta(ctr *Container, netns string) error {
	return pasta.Setup(&pasta.SetupOptions{
		Config:       r.config,
		Netns:        netns,
		Ports:        ctr.convertPortMappings(),
		ExtraOptions: ctr.config.NetworkOptions[pasta.BinaryName],
	})
}
