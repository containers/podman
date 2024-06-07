![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Introduction
This setup assigns a single IPv6 Address to each Application running on the Podman Host, with `podman` running as Normal User (rootless).

An Application can be defined as a set of Containers that share the same Network Namespace.

An Application has all of its Containers located within the same `compose.yml` File.

Typically, for a Web Application, this includes the following:
- A Proxy Server (Caddy)
- An HTTP Application
- (optional) A Database Backend (e.g. PostgreSQL)
- (optional) A Caching Server (e.g. Redis)
- ...

For Remote Clients using IPv6 Connectivity, the Communication with the Container is Direct.

For Remote clients using IPv4 Connectivity, the [snid](https://github.com/AGWA/snid) TLS Proxy Server is Used to achieve an IPv4 <-> IPv6 Translation (NAT46). This required `root` Privileges.

This Tutorial will illustrate how all of this can be achieved using `pasta`, `snid`, `systemd` Services and `podman-compose`.

A similar Result can probably be achieved using Quadlets or Podlets instead of `systemd` and `podman-compose`.

# Pasta Networking
> **Warning**  
> 
> While `pasta` is the Default Networking for rootless `podman` since Podman 5.0, this is NOT the case for `podman-compose` ! Indeed `podman-compose` (at least until and including Version 1.1.0) will default to create a Bridge Network, if `network_mode` is NOT set to `pasta:[list_of_options_for_pasta]`

> **Warning**  
> 
> `traefik` does NOT appear to be working with anything besides rootlessport `bridge` !
> I tried to use `pasta` with `traefik` Container and I kept getting a `service \"dashboard\" error: unable to find the IP address for the container \"/traefik\": the server is ignored).`.

> **Note**  
>
> `podman` does indeed NOT appear to register any IPAddress when using `pasta` Networking, based on `podman inspect <container>`, which might explain why `traefik` is failing.

# Network Conventions within the Tutorial
A lof of Network Conventions are assumed within this Tutorial, with lots of IP Addresses to Remember.

This is Important because in the following Sections it will be explained the Operations to be performed in order to assign/set the required IP Addresses and Routes between Host and Container.

This Tutorial assumes the General "Homelab" Setup (within a LAN/VLAN), or Hosted but sitting behind another Firewall+Router such as OPNSense. In other words, the Podman Host is assumed to be NAT regarding IPv4.

This is NOT required in case you have a Server which has a Public IPv4 Address, however for the sake of Explanation, the NAT Setup is best, especially with regards to logging the Remote Client IPv4 Address.

![PODMAN Pasta Tutorial Network Diagram](https://raw.githubusercontent.com/containers/podman/tree/main/docs/tutorials/podman_pasta_ipv6_with_snid_ipv4.svg)

The Podman Host (Bare Metal or e.g. KVM Virtual Machine) is supposed to have:
- Public IPv4 Address: 12.34.56.78
- Private IPv4 Address: 172.16.1.10/24
- Public+Private IPv6 Address: IPv6: 2a01:XXXX:XXXX:XX01:0000:0000:0000:0100/128

Each Application will furthermore have an IPv6 Address, to which it will bind the Required Ports, which are typically:
- Port 443/tcp (HTTPS)
- Port 443/udp (HTTP3)
- Port 80/tcp (HTTP)

The Applications are supposed to be located within the following Network: 2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001/112 (2a01:XXXX:XXXX:XXXX:0000:0000:0001:0000 ... 2a01:XXXX:XXXX:XXXX:0000:0000:0001:ffff)

The Remote End-Client (Laptop) is supposed to have:
- Public IPv4 Address: 98.76.54.32
- Public+Private IPv6 Address: 2a03:YYYY:YYYY:YY00:0000:0000:0000:0100/64

The IP Addresses of the Routers/Firewalls themselves are not described in this Section, as they are not relevant for the Configuration described by this Tutorial.

> **Note**  
>
> Remember to Open the Required Ports in the Upstream Firewall (e.g. OPNSense, OpenWRT, etc) as well as on the Podman Host, if a Firewall is Enabled (e.g. `firewalld`).

# snid Setup
In order to ensure that IPv4-only Remote Clients can access the Applications running on the Podman Host (Applications which ONLY have an IPv6 Address), an IPv4 <-> IPv6 Translation must be performed.

This Setup assumes that `snid` is installed on the Podman Host itself.

Other Setups where `snid` is running e.g. in a separate KVM Virtual Machine are possible, but require setting up a Static Route from the Podman Host to `64:ff9b:1::/96` (otherwise the Application can be contacted by the Remote Client, but the Application will NOT be able to send any Reply to it).

The easiest way to run `snid` is to download the precompiled Binary from the Official Website and setup a Systemd Service for it. Compiling `snid` from Source it's possible but it involves installing the `go` Development Toolchain, which is NOT the purpose of this Tutorial.

First Download the Program:
```
# Run these Commands on the Podman Host as <root>
mkdir -p /opt/snid
wget https://github.com/AGWA/snid/releases/download/v0.3.0/snid-v0.3.0-linux-amd64 -O /opt/snid/snid
chmod +x /opt/snid/snid
```

Then create a Systemd Service for it in `/etc/systemd/system/snid.service`
```
# To get SNID to work:
# Backend CIDR is supposed to be:
# - Backend CIDR: 2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001/112 (2a01:XXXX:XXXX:XXXX:0000:0000:0001:0000 ... 2a01:XXXX:XXXX:XXXX:0000:0000:0001:ffff)
#
# Convert IPv4 Address to IPv6 Address Representation: 
# - https://www.agwa.name/blog/post/using_sni_proxying_and_ipv6_to_share_port_443
# - https://www.rfc-editor.org/rfc/rfc6052
# - https://github.com/luckylinux/ipv6-decode-ipv4-address

[Unit]
Description=SNID Service

[Service]
# Running as rootless does NOT appear to work, even when adding AmbientCapabilities=CAP_NET_BIND_SERVICE
User=root
ExecStart=/bin/bash -c 'cd /opt/snid && ip route add local 64:ff9b:1::/96 dev lo && ./snid -listen tcp:172.16.1.10:443 -mode nat46 -nat46-prefix 64:ff9b:1:: -backend-cidr 2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001/112'
ExecStop=/bin/bash -c 'cd /opt/snid && ip route del local 64:ff9b:1::/96 dev lo'

[Install]
WantedBy=multi-user.target
```

Reload Systemd, enable and Start the Service:
```
systemctl daemon-reload
systemctl enable snid.service
systemctl restart snid.service
systemctl status snid.service
```

Check that no Errors occurred !

# IPv6 Networking Setup
> **Warning**  
> 
> Each Application has a different IPv6 Address MUST FIRST BE REGISTED ON THE HOST as well. It is NOT possible to just start the Container and expect it to bind to the IP Address configured in `compose.yml` if the IP Address was not registered on the Host in the first Place !

General:
```
ip -6 addr add 2a01:XXXX:XXXX:XX01:0000:0000:0001:0001/64 dev ens18 
```

For Fedora:
```
nmcli connection edit ens18
set ipv6.address
2a01:XXXX:XXXX:XX01:0000:0000:0001:0001
"Do you want to set 'ipv6.method' to manual"? -> no
nmcli connection show ens18 | grep -i addr
systemctl restart NetworkManager
```

For Debian/Ubuntu add in `/etc/network/interfaces` (or `/etc/network/interfaces.d/<my-interface>`) a line within the relevant Interface Block:
```
ip -6 addr add 2a01:XXXX:XXXX:XX01:0000:0000:0001:0001/64 dev ens18
```

# DNS Setup
In Order for Direct IPv6 Connectivity to work, an appropriate AAAA (IPv6) Record Must be present for the Application Hostname:
```
application01          IN      AAAA    2a01:XXXX:XXXX:XX01:0000:0000:0001:0001
```

For IPv4 Connectivity, it is the IPv4 Address of the `snid` Host that must be Entered.
In this Tutorial, since `snid` is assumed to be running on the Podman Host itself, this simply means creating an A Record for the Podman Host itself:
```
application01          IN      A    12.34.56.78
```

> **Warning**  
> 
> Issues can arise due to DNSSec and/or DNS over TLS Configuration related Issues. Try to disable those, flush caches with `resolvectl flush-caches` and issue `systemctl restart systemd-resolved` to see if the Issue disappears.

# Compose File Setup
Two different Approaches for the Compose File are Possible and both Work.

## Compose with Port Mapping + Minimal Pasta Line
With this Method, `podman ps` will show the Open Ports in its Output.

```
services:
  caddy:
    image: caddy:latest
    #image: lucaslorentz/caddy-docker-proxy:2.9-alpine
    pull_policy: "missing"
    container_name: caddy
    restart: "unless-stopped"
    security_opt:
      - no-new-privileges:true
      - label=type:container_runtime_t
    ports:
      - target: 80
        host_ip: "[2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001]"
        published: 80
        protocol: tcp
      - target: 443
        host_ip: "[2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001]"
        published: 443
        protocol: tcp
      - target: 443
        host_ip: "[2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001]"
        published: 443
        protocol: udp
    network_mode: "pasta:--ipv6-only"
    volumes:
 #     - /run/user/1001/podman/podman.sock:/var/run/docker.sock:rw,z
      - ./Caddyfile:/etc/caddy/Caddyfile:ro,z
      - ~/containers/local/caddy:/srv:ro,z
      - ~/containers/data/caddy:/data:rw,z
      - ~/containers/log/caddy:/var/log:rw,z
      - ~/containers/config/caddy:/config:rw,z
      - ~/containers/certificates/letsencrypt:/certificates:ro,z
    environment:
      - CADDY_DOCKER_CADDYFILE_PATH=/etc/caddy/Caddyfile

  # Proxy to container
  whoami:
    image: traefik/whoami
    pull_policy: "missing"
    container_name: whoami
    restart: "unless-stopped"
    network_mode: "service:caddy"
    environment:
      - WHOAMI_PORT_NUMBER=8080
```

## Compose with only Pasta Line
With this Method, `podman ps` will NOT show the Open Ports in its Output.

In fact, `podman inspect <container>` will NOT list the IP Addresses nor where the Ports are bound to.

This works correctly, but the only way to examine if the Port is used is to run `ss -nlt6`.

```
services:
  caddy:
    image: caddy:latest
    #image: lucaslorentz/caddy-docker-proxy:2.9-alpine
    pull_policy: "missing"
    container_name: caddy
    restart: "unless-stopped"
    security_opt:
      - no-new-privileges:true
      - label=type:container_runtime_t
    network_mode: "pasta:--ipv6-only,-t,2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001/80,-t,2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001/443,-u,2a01:XXXX:XXXX:XXXX:0000:0000:0001:0001/443"
    volumes:
 #     - /run/user/1001/podman/podman.sock:/var/run/docker.sock:rw,z
      - ./Caddyfile:/etc/caddy/Caddyfile:ro,z
      - ~/containers/local/caddy:/srv:ro,z
      - ~/containers/data/caddy:/data:rw,z
      - ~/containers/log/caddy:/var/log:rw,z
      - ~/containers/config/caddy:/config:rw,z
      - ~/containers/certificates/letsencrypt:/certificates:ro,z
    environment:
      - CADDY_DOCKER_CADDYFILE_PATH=/etc/caddy/Caddyfile

  # Proxy to container
  whoami:
    image: traefik/whoami
    pull_policy: "missing"
    container_name: whoami
    restart: "unless-stopped"
    network_mode: "service:caddy"
    environment:
      - WHOAMI_PORT_NUMBER=8080
```

> **Warning**  
> 
> Netstat is deprecated. `netstat -an | grep -i listen` will NOT return the correct IPv6 Addresses in most cases, because it truncates the Output. `netstat -Wan | grep -i listen` will return the full IPv6 Address wthout truncating it, but you should really be using `ss -nlt6` instead.

# Caddy Proxy Configuration
For Simple Configurations of Applications and automatically generating a SSL Certificate using Letsencrypt with the HTTP(S) Challenge, one can just define `command` within the `compose.yml` File to something like:
```
command: reverse-proxy --from application01.MYDOMAIN.TLD --to 'https//::1:8080'
```

For semi-automated Setups, one can use the `lucaslorentz/caddy-docker-proxy` Docker Image, which allows to set Caddy Options directly within the `compose.yml` File.

I am NOT used to this Syntax, so I am just using the Caddyfile, at least for now.
This is also due to the Fact that I am self-managing the Letsencrypt Certificates using `certbot` and distributing them across my Infrastructure.

Your mileage may vary :).

```
# Example and Guide
# https://caddyserver.com/docs/caddyfile/options

# General Options
{
    # Debug Mode
    debug

    # Ports Configuration
    #http_port 80
    #https_port 443

    # TLS Options
    auto_https disable_certs

    # Default SNI
    default_sni MYDOMAIN.TLD
}


localhost {
	reverse_proxy /api/* localhost:9001
}

application01.MYDOMAIN.TLD {
        tls /certificates/MYDOMAIN.TLD/fullchain.pem /certificates/MYDOMAIN.TLD/privkey.pem
        log {
		output file /var/log/application01.MYDOMAIN.TLD/access.json {
			roll_size 100MiB
			roll_keep 5000
			roll_keep_for 720h
            roll_uncompressed
		}
        format json
	}

    reverse_proxy http://[::1]:8080
}
```

# Run the Application
Simply Run
```
podman-compose up -d
```

In case of Issues, you might want to Debug with DEBUG log level and disabling detached mode:
```
podman-compose --podman-run-args="--log-level=debug" up
```

# Testing
From a Remote Client (NOT located withing the same LAN, try to use 4G/LTE Connectivity otherwise) test that Connectivity is working.

It is reccomended to test against something like `traefik/whoami` Application (as described in this Tutorial's `compose.yml` File), which can display many Parameters, including HTTP and especially X-Forwarded-For Headers.

IPv6 Test
```
curl --vvv -6 application01.MYDOMAIN.TLD
```

IPv4 Test
```
curl --vvv -4 application01.MYDOMAIN.TLD
```

In case of Issues in the IPv4 Test, check the `snid` Service Status for Clues:
```
systemctl status snid.service
journalctl -xeu snid.service
```

You might also want to check the `caddy` Proxy Logs for other Clues:
```
podman logs caddy
cat ~/containers/log/caddy/application01.MYDOMAIN.TLD/access.json | jq -r
```

# Translating IPv6 to IPv4 based on Logs
This [Tool](https://github.com/luckylinux/ipv6-decode-ipv4-address) can be used to Translate IPv4-Embedded Addresses (within an IPv6 Address, done by `snid` through `NAT46`) back to the Original IPv4 Address.

Simply read the Logs and look for `request`, especially `client_ip` and `remote_ip`.