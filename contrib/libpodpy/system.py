
class System(object):
    def __init__(self, client):
        self.client = client

    def Ping(self):
        return self.client.Ping()

    def Version(self):
        return self.client.GetVersion()
