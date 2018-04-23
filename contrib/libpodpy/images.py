
class Images(object):
    """
    The Images class deals with image related functions for libpod.
    """

    def __init__(self, client):
        self.client = client

    def List(self):
        """
        Lists all images in the libpod image store
        return: a list of ImageList objects
        """
        return self.client.ListImages()
