from docker import APIClient

from test.python.docker import constant


def run_top_container(client: APIClient):
    c = client.create_container(
        constant.ALPINE, command="top", detach=True, tty=True, name="top"
    )
    client.start(c.get("Id"))
    return c.get("Id")


def remove_all_containers(client: APIClient):
    for ctnr in client.containers(quiet=True):
        client.remove_container(ctnr, force=True)


def remove_all_images(client: APIClient):
    for image in client.images(quiet=True):
        client.remove_image(image, force=True)
