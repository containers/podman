#!/usr/bin/env python3
"""Remote podman client."""

import argparse
import curses
import getpass
import inspect
import logging
import os
import sys

import pkg_resources

import lib.actions
import pytoml

assert lib.actions  # silence pyflakes

# TODO: setup.py and obtain __version__ from rpm.spec
try:
    __version__ = pkg_resources.get_distribution('pydman').version
except Exception:
    __version__ = '0.0.0'


class HelpFormatter(argparse.RawDescriptionHelpFormatter):
    """Set help width to screen size."""

    def __init__(self, *args, **kwargs):
        """Construct HelpFormatter using screen width."""
        if 'width' not in kwargs:
            kwargs['width'] = 80
            try:
                height, width = curses.initscr().getmaxyx()
                kwargs['width'] = width
            finally:
                curses.endwin()
        super().__init__(*args, **kwargs)


class PodmanArgumentParser(argparse.ArgumentParser):
    """Default remote podman configuration."""

    def __init__(self, **kwargs):
        """Construct the parser."""
        kwargs['add_help'] = True
        kwargs['allow_abbrev'] = True
        kwargs['description'] = __doc__
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
            default='INFO',
            type=str.upper,
            help='set logging level for events. (default: %(default)s)',
        )
        self.add_argument(
            '--run-dir',
            help=('directory to place local socket bindings.'
                  ' (default: XDG_RUNTIME_DIR)'))
        self.add_argument(
            '--user',
            help=('Authenicating user on remote host.'
                  ' (default: {})').format(getpass.getuser()))
        self.add_argument(
            '--host', help='name of remote host. (default: None)')
        self.add_argument(
            '--remote-socket-path',
            help=('path of podman socket on remote host'
                  ' (default: /run/podman/io.projectatomic.podman)'))
        self.add_argument(
            '--identity-file',
            help=('path to ssh identity file. (default: ~/.ssh/id_rsa)'))
        self.add_argument(
            '--config',
            default='/etc/containers/podman_client.conf',
            dest='config_file',
            help='path of configuration file. (default: %(default)s)')

        actions_parser = self.add_subparsers(
            dest='subparser_name', help='actions')

        for name, obj in inspect.getmembers(
                sys.modules['lib.actions'],
                lambda member: inspect.isclass(member)):
            if hasattr(obj, 'subparser'):
                try:
                    obj.subparser(actions_parser)
                except NameError as e:
                    logging.critical(e)
                    logging.warning(
                        'See subparser configuration for Class "{}"'.format(
                            name))
                    sys.exit(3)

    def parse_args(self, args=None, namespace=None):
        """Parse command line arguments, backed by env var and config_file."""
        self.initialize_parser()
        cooked = super().parse_args(args, namespace)
        return self.resolve_configuration(cooked)

    def resolve_configuration(self, args):
        """Find and fill in any arguments not passed on command line."""
        try:
            # Configuration file optionall, arguments may be provided elsewhere
            with open(args.config_file, 'r') as stream:
                config = pytoml.load(stream)
        except OSError:
            logging.info(
                'Failed to read: {}'.format(args.config_file),
                exc_info=args.log_level == logging.DEBUG)
            config = {'default': {}}
        else:
            if 'default' not in config:
                config['default'] = {}

        def resolve(name, value):
            if value:
                setattr(args, name, value)
                return value
            self.error('Required argument "%s" is not configured.' % name)

        xdg = os.path.join(os.environ['XDG_RUNTIME_DIR'], 'podman') \
            if os.environ.get('XDG_RUNTIME_DIR') else None

        resolve(
            'run_dir',
            getattr(args, 'run_dir')
            or os.environ.get('RUN_DIR')
            or config['default'].get('run_dir')
            or xdg
            or '/tmp/podman' if os.path.isdir('/tmp') else None
        )   # yapf: disable

        args.local_socket_path = os.path.join(args.run_dir, "podman.socket")

        resolve(
            'host',
            getattr(args, 'host')
            or os.environ.get('HOST')
            or config['default'].get('host')
        )   # yapf:disable

        resolve(
            'user',
            getattr(args, 'user')
            or os.environ.get('USER')
            or config['default'].get('user')
            or getpass.getuser()
        )   # yapf:disable

        resolve(
            'remote_socket_path',
            getattr(args, 'remote_socket_path')
            or os.environ.get('REMOTE_SOCKET_PATH')
            or config['default'].get('remote_socket_path')
            or '/run/podman/io.projectatomic.podman'
        )   # yapf:disable

        resolve(
            'identity_file',
            getattr(args, 'identity_file')
            or os.environ.get('IDENTITY_FILE')
            or config['default'].get('identity_file')
            or os.path.expanduser('~{}/.ssh/id_rsa'.format(args.user))
        )   # yapf:disable

        args.local_uri = "unix:{}".format(args.local_socket_path)
        args.remote_uri = "ssh://{}@{}{}".format(args.user, args.host,
                                                 args.remote_socket_path)
        return args

    def exit(self, status=0, message=None):
        """Capture message and route to logger."""
        if message:
            log = logging.info if status == 0 else logging.error
            log(message)
        super().exit(status)

    def error(self, message):
        """Capture message and route to logger."""
        logging.error('{}: {}'.format(self.prog, message))
        logging.error("Try '{} --help' for more information.".format(
            self.prog))
        super().exit(2)


if __name__ == '__main__':
    # Setup logging so we use stderr and can change logging level later
    # Do it now before there is any chance of a default setup.
    log = logging.getLogger()
    fmt = logging.Formatter('%(asctime)s | %(levelname)-8s | %(message)s',
                            '%Y-%m-%d %H:%M:%S %Z')
    stderr = logging.StreamHandler(stream=sys.stderr)
    stderr.setFormatter(fmt)
    log.addHandler(stderr)
    log.setLevel(logging.INFO)

    parser = PodmanArgumentParser()
    args = parser.parse_args()

    log.setLevel(args.log_level)
    logging.debug('Logging initialized at level {}'.format(
        logging.getLevelName(logging.getLogger().getEffectiveLevel())))

    def istraceback():
        """Add traceback when logging events."""
        return log.getEffectiveLevel() == logging.DEBUG

    try:
        if not os.path.exists(args.run_dir):
            os.makedirs(args.run_dir)
    except PermissionError as e:
        logging.critical(e, exc_info=istraceback())
        sys.exit(6)

    # Klass(args).method() are setup by the sub-command's parser
    returncode = None
    try:
        obj = args.klass(args)
    except Exception as e:
        logging.critical(repr(e), exc_info=istraceback())
        logging.warning('See subparser "{}" configuration.'.format(
            args.subparser_name))
        sys.exit(5)

    try:
        returncode = getattr(obj, args.method)()
    except AttributeError as e:
        logging.critical(e, exc_info=istraceback())
        logging.warning('See subparser "{}" configuration.'.format(
            args.subparser_name))
        returncode = 3
    except (ConnectionResetError, TimeoutError) as e:
        logging.critical(e, exc_info=istraceback())
        logging.info('Review connection arguments for correctness.')
        returncode = 4

    sys.exit(0 if returncode is None else returncode)
