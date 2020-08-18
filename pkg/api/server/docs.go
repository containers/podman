// Package api Provides a container compatible interface. (Experimental)
//
// This documentation describes the HTTP Libpod interface.  It is to be considered
// only as experimental as this point.  The endpoints, parameters, inputs, and
// return values can all change.
//
// To start the service and keep it running for 5,000 seconds (-t 0 runs forever):
//
//    podman system service -t 5000 &
//
// You can then use cURL on the socket using requests documented below.
//
// NOTE: if you install the package podman-docker, it will create a symbolic
// link for /var/run/docker.sock to /run/podman/podman.sock
//
// See podman-service(1) for more information.
//
//  Quick Examples:
//
//   'podman info'
//
//      curl --unix-socket /run/podman/podman.sock http://d/v1.0.0/libpod/info
//
//   'podman pull quay.io/containers/podman'
//
//      curl -XPOST --unix-socket /run/podman/podman.sock -v 'http://d/v1.0.0/images/create?fromImage=quay.io%2Fcontainers%2Fpodman'
//
//   'podman list images'
//
//      curl --unix-socket /run/podman/podman.sock -v 'http://d/v1.0.0/libpod/images/json' | jq
//
// Terms Of Service:
//
//     Schemes: http, https
//     Host: podman.io
//     BasePath: /
//     Version: 0.0.1
//     License: Apache-2.0 https://opensource.org/licenses/Apache-2.0
//     Contact: Podman <podman@lists.podman.io> https://podman.io/community/
//
//     InfoExtensions:
//     x-logo:
//       - url: https://raw.githubusercontent.com/containers/podman/master/logo/podman-logo.png
//       - altText: "Podman logo"
//
//     Produces:
//     - application/json
//     - text/plain
//     - text/html
//
//     Consumes:
//     - application/json
//     - application/x-tar
// swagger:meta
package server
