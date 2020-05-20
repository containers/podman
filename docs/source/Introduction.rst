.. include:: includes.rst

Introduction
==================================
Containers_ simplify the consumption of applications with all of their dependencies and default configuration files. Users test drive or deploy a new application with one or two commands instead of following pages of installation instructions. Here's how to find your first `Container Image`_::

    podman search busybox

Output::

    INDEX       NAME                                DESCRIPTION                                       STARS   OFFICIAL   AUTOMATED
    docker.io   docker.io/library/busybox           Busybox base image.                               1882    [OK]
    docker.io   docker.io/radial/busyboxplus        Full-chain, Internet enabled, busybox made f...   30                 [OK]
    docker.io   docker.io/yauritux/busybox-curl     Busybox with CURL                                 8
    ...

The previous command returned a list of publicly available container images on DockerHub. These container images are easy to consume, but of differing levels of quality and maintenance. Let’s use the first one listed because it seems to be well maintained.

To run the busybox container image, it’s just a single command::

    podman run -it docker.io/library/busybox

Output::

    / #

You can poke around in the busybox container for a while, but you’ll quickly find that running small container with a few Linux utilities in it provides limited value, so exit out::

    exit

There’s an old saying that “nobody runs an operating system just to run an operating system” and the same is true with containers. It’s the workload running on top of an operating system or in a container that’s interesting and valuable.

Sometimes we can find a publicly available container image for the exact workload we’re looking for and it will already be packaged exactly how we want. But, more often than not, there’s something that we want to add, remove, or customize. It could be as simple as a configuration setting for security or performance, or as complex as adding a complex workload. Either way, containers make it fairly easy to make the changes we need.

Container Images aren’t actually images, they’re repositories often made up of multiple layers. These layers can easily be added, saved, and shared with others by using a Containerfile (Dockerfile). This single file often contains all the instructions needed to build the new and can easily be shared with others publicly using tools like GitHub.

Here's an example of how to build an Nginx web server on top of a Debian base image using the Dockerfile maintained by Nginx and published in GitHub::

    podman build -t nginx https://git.io/Jf8ol

Once, the image build completes, it’s easy to run the new image from our local cache::

    podman run -d -p 8080:80 nginx
    curl localhost:8080

Output::

    ...
    <p><em>Thank you for using nginx.</em></p>
    ...

Building new images is great, but sharing our work with others let’s them review our work, critique how we built them, and offer improved versions. Our newly built Nginx image could be published at quay.io or docker.io to share it with the world. Everything needed to run the Nginx application is provided in the container image. Others could easily pull it down and use it, or make improvements to it.

Standardizing on container images and `Container Registries`_ enable a new level of collaboration through simple consumption. This simple consumption model is possible because every major Container Engine and Registry Server uses the Open Containers Initiative (OCI_) format. This allows users to find, run, build, share and deploy containers anywhere they want. Podman and other `Container Engines`_ like CRI-O, Docker, or containerd can create and consume container images from docker.io, quay.io, an on premise registry or even one provided by a cloud provider. The OCI image format facilitates this ecosystem through a single standard.

For example, if we wanted to share our newly built Nginx container image on quay.io it’s easy. First log in to quay::

    podman login quay.io
Input::

    Username: USERNAME
    Password: ********
    Login Succeeded!

Nex, tag the image so that we can push it into our user account::

    podman tag localhost/nginx quay.io/USERNAME/nginx

Finally, push the image::

    podman push quay.io/USERNAME/nginx

Output::

    Getting image source signatures
    Copying blob 38c40d6c2c85 done
    Copying blob fee76a531659 done
    Copying blob c2adabaecedb done
    Copying config 7f3589c0b8 done
    Writing manifest to image destination
    Copying config 7f3589c0b8 done
    Writing manifest to image destination
    Storing signatures

Notice that we pushed four layers to our registry and now it’s available for others to share. Take a quick look::

    podman inspect quay.io/USERNAME/nginx

Output::

    [
        {
            "Id": "7f3589c0b8849a9e1ff52ceb0fcea2390e2731db9d1a7358c2f5fad216a48263",
            "Digest": "sha256:7822b5ba4c2eaabdd0ff3812277cfafa8a25527d1e234be028ed381a43ad5498",
            "RepoTags": [
                "quay.io/USERNAME/nginx:latest",
    ...

To summarize, Podman makes it easy to find, run, build and share containers.

* Find: whether finding a container on dockerhub.io or quay.io, an internal registry server, or directly from a vendor, a couple of `podman search`_, and `podman pull`_ commands make it easy
* Run: it's easy to consume pre-built images with everything needed to run an entire application, or start from a Linux distribution base image with the `podman run`_ command
* Build: creating new layers with small tweaks, or major overhauls is easy with `podman build`
* Share: Podman let’s you push your newly built containers anywhere you want with a single `podman push`_ command

For more instructions on use cases, take a look at our :doc:`Tutorials` page.
