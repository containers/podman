"""Remote client command for reporting on Podman service."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Version(AbstractActionBase):
    """Class for reporting on Podman Service."""

    @classmethod
    def subparser(cls, parent):
        """Add Version command to parent parser."""
        parser = parent.add_parser(
            'version', help='report version on podman service')
        parser.set_defaults(class_=cls, method='version')

    def version(self):
        """Report on Podman Service."""
        try:
            info = self.client.system.info()
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        else:
            version = info._asdict()['podman']
            host = info._asdict()['host']
            print("Version        {}".format(version['podman_version']))
            print("Go Version     {}".format(version['go_version']))
            print("Git Commit     {}".format(version['git_commit']))
            print("OS/Arch        {}/{}".format(host["os"], host["arch"]))
