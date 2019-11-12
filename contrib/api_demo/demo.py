#!/usr/bin/env python3

import docker, json
import urllib.request
import urllib.parse

client = docker.from_env()

#image = client.images.pull("quay.io/libpod/alpine_nginx", tag="latest")

#client.pull("quay.io/libpod/alpine_nginx", tag="latest")
#client.tag("quay.io/libpod/alpine_nginx:latest", "foobar")

#for img in client.images.list(all=True):
#    print('{0:35} {1:12}'.format("NAME", "ID"))
#    for repotag in img.tags:
#        print('{0:35} {1:12}'.format(repotag, img.id[0:12]))
#
#img = client.images.get(image.id)
#response = client.containers.create(image="foobar", command="nginx -g 'daemon off';", name="foobar", network="host")
#
#ctr = client.containers.get(response.id)
#
#ctr.start()

#except json.decoder.JSONDecodeError:
#    print("catch but moving on")
#    print(response)

#inspect = client.containers.get()


#url = 'https://api.spotify.com/v1/search?type=artist&q=snoop'
#f = urllib.request.urlopen(url)
#print(f.read().decode('utf-8'))


print('{0:35} {1:12}'.format("NAME", "ID"))
for con in client.containers.list():
    print('{0:35} {1:12}'.format("", con.short_id))
