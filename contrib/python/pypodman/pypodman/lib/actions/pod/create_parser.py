"""Remote client command for creating pod."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class CreatePod(AbstractActionBase):
    """Implement Create Pod command."""

    @classmethod
    def subparser(cls, parent):
        """Add Pod Create command to parent parser."""
        parser = parent.add_parser('create', help='create pod')
        super().subparser(parser)

        parser.add_argument(
            '--cgroup-parent',
            dest='cgroupparent',
            type=str,
            help='Path to cgroups under which the'
            ' cgroup for the pod will be created.')
        parser.add_flag(
            '--infra',
            help='Create an infra container and associate it with the pod.')
        parser.add_argument(
            '-l',
            '--label',
            dest='labels',
            action='append',
            type=str,
            help='Add metadata to a pod (e.g., --label=com.example.key=value)')
        parser.add_argument(
            '-n',
            '--name',
            dest='ident',
            type=str,
            help='Assign name to the pod')
        parser.add_argument(
            '--share',
            choices=('ipc', 'net', 'pid', 'user', 'uts'),
            help='Comma deliminated list of kernel namespaces to share')

        parser.set_defaults(class_=cls, method='create')

        # TODO: Add golang CLI arguments not included in API.
        # parser.add_argument(
        #     '--infra-command',
        #     default='/pause',
        #     help='Command to run to start the infra container.'
        #     '(default: %(default)s)')
        # parser.add_argument(
        #     '--infra-image',
        #     default='k8s.gcr.io/pause:3.1',
        #     help='Image to create for the infra container.'
        #     '(default: %(default)s)')
        # parser.add_argument(
        #     '--podidfile',
        #     help='Write the pod ID to given file name on remote host')

    def create(self):
        """Create Pod from given options."""
        config = {}
        for key in ('ident', 'cgroupparent', 'infra', 'labels', 'share'):
            config[key] = self.opts.get(key)

        try:
            pod = self.client.pods.create(**config)
        except podman.ErrorOccurred as ex:
            sys.stdout.flush()
            print(
                '{}'.format(ex.reason).capitalize(),
                file=sys.stderr,
                flush=True)
        else:
            print(pod.id)
