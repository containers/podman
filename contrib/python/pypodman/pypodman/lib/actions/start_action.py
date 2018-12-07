"""Remote client command for starting containers."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Start(AbstractActionBase):
    """Class for starting container."""

    @classmethod
    def subparser(cls, parent):
        """Add Start command to parent parser."""
        parser = parent.add_parser('start', help='start container')
        parser.add_flag(
            '--attach',
            '-a',
            help="Attach container's STDOUT and STDERR.")
        parser.add_argument(
            '--detach-keys',
            metavar='KEY(s)',
            default=4,
            help='Override the key sequence for detaching a container.'
            ' (format: a single character [a-Z] or ctrl-<value> where'
            ' <value> is one of: a-z, @, ^, [, , or _) (default: ^D)')
        parser.add_flag(
            '--interactive',
            '-i',
            help="Attach container's STDIN.")
        # TODO: Implement sig-proxy
        parser.add_flag(
            '--sig-proxy',
            help="Proxy received signals to the process."
        )
        parser.add_argument(
            'containers',
            nargs='+',
            help='containers to start',
        )
        parser.set_defaults(class_=cls, method='start')

    def start(self):
        """Start provided containers."""
        stdin = sys.stdin if self.opts['interactive'] else None
        stdout = sys.stdout if self.opts['attach'] else None

        try:
            for ident in self._args.containers:
                try:
                    ctnr = self.client.containers.get(ident)
                    ctnr.attach(
                        eot=self.opts['detach_keys'],
                        stdin=stdin,
                        stdout=stdout)
                    ctnr.start()
                except podman.ContainerNotFound as e:
                    sys.stdout.flush()
                    print(
                        'Container "{}" not found'.format(e.name),
                        file=sys.stderr,
                        flush=True)
                else:
                    print(ident)
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        return 0
