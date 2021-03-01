from docker import DockerClient

from test.python.docker.compat import constant


def run_top_container(client: DockerClient):
    c = client.containers.create(
        constant.ALPINE, command="top", detach=True, tty=True, name="top"
    )
    c.start()
    return c.id


def remove_all_containers(client: DockerClient):
    for ctnr in client.containers.list(all=True):
        ctnr.remove(force=True)


def remove_all_images(client: DockerClient):
    for img in client.images.list():
        # FIXME should DELETE /images accept the sha256: prefix?
        id_ = img.id.removeprefix("sha256:")
        client.images.remove(id_, force=True)
