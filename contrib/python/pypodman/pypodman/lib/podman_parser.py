"""Parse configuration while building subcommands."""
import argparse
import getpass
import inspect
import logging
import os
import shutil
import sys
from contextlib import suppress
from pathlib import Path

import pkg_resources
import pytoml

from .parser_actions import PathAction, PositiveIntAction

# TODO: setup.py and obtain __version__ from rpm.spec
try:
    __version__ = pkg_resources.get_distribution('pypodman').version
except Exception:  # pylint: disable=broad-except
    __version__ = '0.0.0'


class HelpFormatter(argparse.RawDescriptionHelpFormatter):
    """Set help width to screen size."""

    def __init__(self, *args, **kwargs):
        """Construct HelpFormatter using screen width."""
        if 'width' not in kwargs:
            try:
                size = shutil.get_terminal_size()
                kwargs['width'] = size.columns
            except Exception:  # pylint: disable=broad-except
                kwargs['width'] = 80

        super().__init__(*args, **kwargs)


class PodmanArgumentParser(argparse.ArgumentParser):
    """Default remote podman configuration."""

    def __init__(self, **kwargs):
        """Construct the parser."""
        kwargs['add_help'] = True
        kwargs['description'] = ('Portable and simple management'
                                 ' tool for containers and images')
        kwargs['formatter_class'] = HelpFormatter

        super().__init__(**kwargs)

    def initialize_parser(self):
        """Initialize parser without causing recursion meltdown."""
        self.add_argument(
            '--version',
            action='version',
            version='%(prog)s v. ' + __version__)
        self.add_argument(
            '--log-level',
            choices=['DEBUG', 'INFO', 'WARNING', 'ERROR', 'CRITICAL'],
            default='WARNING',
            type=str.upper,
            help='set logging level for events. (default: %(default)s)',
        )
        self.add_argument(
            '--run-dir',
            metavar='DIRECTORY',
            help=('directory to place local socket bindings.'
                  ' (default: XDG_RUNTIME_DIR/pypodman)'))
        self.add_argument(
            '--username',
            '-l',
            help='Authenicating user on remote host. (default: {})'.format(
                getpass.getuser()))
        self.add_argument(
            '--host', help='name of remote host. (default: None)')
        self.add_argument(
            '--port',
            '-p',
            action=PositiveIntAction,
            help='port for ssh tunnel to remote host. (default: 22)')
        self.add_argument(
            '--remote-socket-path',
            metavar='PATH',
            help=('path of podman socket on remote host'
                  ' (default: /run/podman/io.podman)'))
        self.add_argument(
            '--identity-file',
            '-i',
            action=PathAction,
            help='path to ssh identity file. (default: ~user/.ssh/id_dsa)')
        self.add_argument(
            '--ignore-hosts',
            action='store_true',
            help='ignore ssh host keys (default: False)')
        self.add_argument(
            '--known-hosts',
            '-k',
            action=PathAction,
            help='path to ssh known hosts. (default: ~user/.ssh/known_hosts)')
        self.add_argument(
            '--config-home',
            metavar='DIRECTORY',
            action=PathAction,
            help=('home of configuration "pypodman.conf".'
                  ' (default: XDG_CONFIG_HOME/pypodman)'))

        actions_parser = self.add_subparsers(
            dest='subparser_name', help='commands')

        # import buried here to prevent import loops
        import pypodman.lib.actions  # pylint: disable=cyclic-import
        assert pypodman.lib.actions

        # pull in plugin(s) code for each subcommand
        for name, obj in inspect.getmembers(
                sys.modules['pypodman.lib.actions'],
                predicate=inspect.isclass):
            if hasattr(obj, 'subparser'):
                try:
                    obj.subparser(actions_parser)
                except NameError as e:
                    logging.critical(e)
                    logging.warning(
                        'See subparser configuration for Class "%s"', name)
                    sys.exit(3)

    def parse_args(self, args=None, namespace=None):
        """Parse command line arguments, backed by env var and config_file."""
        self.initialize_parser()
        cooked = super().parse_args(args, namespace)
        return self.resolve_configuration(cooked)

    def resolve_configuration(self, args):
        """Find and fill in any arguments not passed on command line."""
        args.xdg_runtime_dir = os.environ.get('XDG_RUNTIME_DIR', '/tmp')
        args.xdg_config_home = os.environ.get('XDG_CONFIG_HOME',
                                              os.path.expanduser('~/.config'))
        args.xdg_config_dirs = os.environ.get('XDG_CONFIG_DIRS', '/etc/xdg')

        # Configuration file(s) are optional,
        #   required arguments may be provided elsewhere
        config = {'default': {}}
        dirs = args.xdg_config_dirs.split(':')
        dirs.extend((args.xdg_config_home, args.config_home))
        for dir_ in dirs:
            if dir_ is None:
                continue
            with suppress(OSError):
                cnf = Path(dir_, 'pypodman', 'pypodman.conf')
                with cnf.open() as stream:
                    config.update(pytoml.load(stream))

        def reqattr(name, value):
            """Raise an error if value is unset."""
            if value:
                setattr(args, name, value)
                return value
            return self.error(
                'Required argument "{}" is not configured.'.format(name))

        reqattr(
            'run_dir',
            getattr(args, 'run_dir')
            or os.environ.get('PODMAN_RUN_DIR')
            or config['default'].get('run_dir')
            or str(Path(args.xdg_runtime_dir, 'pypodman'))
        )   # yapf: disable

        setattr(
            args,
            'host',
            getattr(args, 'host')
            or os.environ.get('PODMAN_HOST')
            or config['default'].get('host')
        )   # yapf:disable

        reqattr(
            'username',
            getattr(args, 'username')
            or os.environ.get('PODMAN_USER')
            or config['default'].get('username')
            or os.environ.get('USER')
            or os.environ.get('LOGNAME')
            or getpass.getuser()
        )   # yapf:disable

        reqattr(
            'port',
            getattr(args, 'port')
            or os.environ.get('PODMAN_PORT')
            or config['default'].get('port', None)
            or 22
        )   # yapf:disable

        reqattr(
            'remote_socket_path',
            getattr(args, 'remote_socket_path')
            or os.environ.get('PODMAN_REMOTE_SOCKET_PATH')
            or config['default'].get('remote_socket_path')
            or '/run/podman/io.podman'
        )   # yapf:disable

        reqattr(
            'log_level',
            getattr(args, 'log_level')
            or os.environ.get('PODMAN_LOG_LEVEL')
            or config['default'].get('log_level')
            or logging.WARNING
        )  # yapf:disable

        setattr(
            args,
            'identity_file',
            getattr(args, 'identity_file')
            or os.environ.get('PODMAN_IDENTITY_FILE')
            or config['default'].get('identity_file')
            or os.path.expanduser('~{}/.ssh/id_dsa'.format(args.username))
        )   # yapf:disable

        if not os.path.isfile(args.identity_file):
            args.identity_file = None

        setattr(
            args,
            'ignore_hosts',
            getattr(args, 'ignore_hosts')
            or os.environ.get('PODMAN_IGNORE_HOSTS')
            or config['default'].get('ignore_hosts')
            or False
        )   # yapf:disable

        setattr(
            args,
            'known_hosts',
            getattr(args, 'known_hosts')
            or os.environ.get('PODMAN_KNOWN_HOSTS')
            or config['default'].get('known_hosts')
            or os.path.expanduser('~{}/.ssh/known_hosts'.format(args.username))
        )   # yapf:disable

        if not os.path.isfile(args.known_hosts):
            args.known_hosts = None

        if args.host:
            args.local_socket_path = str(Path(args.run_dir, 'podman.socket'))
        else:
            args.local_socket_path = args.remote_socket_path

        args.local_uri = 'unix:{}'.format(args.local_socket_path)

        if args.host:
            components = ['ssh://', args.username, '@', args.host]
            if args.port:
                components.extend((':', str(args.port)))
            components.append(args.remote_socket_path)

            args.remote_uri = ''.join(components)
        return args

    def exit(self, status=0, message=None):
        """Capture message and route to logger."""
        if message:
            log = logging.info if status == 0 else logging.error
            log(message)
        super().exit(status)

    def error(self, message):
        """Capture message and route to logger."""
        logging.error('%s: %s', self.prog, message)
        logging.error("Try '%s --help' for more information.", self.prog)
        super().exit(2)
