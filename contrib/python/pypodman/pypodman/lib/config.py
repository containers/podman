"""Parse configuration while building subcommands."""
import argparse
import curses
import getpass
import inspect
import logging
import os
import sys

import pkg_resources
import pytoml

# TODO: setup.py and obtain __version__ from rpm.spec
try:
    __version__ = pkg_resources.get_distribution('pypodman').version
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
            default='WARNING',
            type=str.upper,
            help='set logging level for events. (default: %(default)s)',
        )
        self.add_argument(
            '--run-dir',
            metavar='DIRECTORY',
            help=('directory to place local socket bindings.'
                  ' (default: XDG_RUNTIME_DIR/pypodman'))
        self.add_argument(
            '--user',
            default=getpass.getuser(),
            help='Authenicating user on remote host. (default: %(default)s)')
        self.add_argument(
            '--host', help='name of remote host. (default: None)')
        self.add_argument(
            '--remote-socket-path',
            metavar='PATH',
            help=('path of podman socket on remote host'
                  ' (default: /run/podman/io.projectatomic.podman)'))
        self.add_argument(
            '--identity-file',
            metavar='PATH',
            help=('path to ssh identity file. (default: ~user/.ssh/id_dsa)'))
        self.add_argument(
            '--config-home',
            metavar='DIRECTORY',
            help=('home of configuration "pypodman.conf".'
                  ' (default: XDG_CONFIG_HOME/pypodman'))

        actions_parser = self.add_subparsers(
            dest='subparser_name', help='actions')

        # pull in plugin(s) code for each subcommand
        for name, obj in inspect.getmembers(
                sys.modules['pypodman.lib.actions'],
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
            try:
                with open(os.path.join(dir_, 'pypodman/pypodman.conf'),
                          'r') as stream:
                    config.update(pytoml.load(stream))
            except OSError:
                pass

        def reqattr(name, value):
            if value:
                setattr(args, name, value)
                return value
            self.error('Required argument "%s" is not configured.' % name)

        reqattr(
            'run_dir',
            getattr(args, 'run_dir')
            or os.environ.get('RUN_DIR')
            or config['default'].get('run_dir')
            or os.path.join(args.xdg_runtime_dir, 'pypodman')
        )   # yapf: disable

        setattr(
            args,
            'host',
            getattr(args, 'host')
            or os.environ.get('HOST')
            or config['default'].get('host')
        )   # yapf:disable

        reqattr(
            'user',
            getattr(args, 'user')
            or os.environ.get('USER')
            or config['default'].get('user')
            or getpass.getuser()
        )   # yapf:disable

        reqattr(
            'remote_socket_path',
            getattr(args, 'remote_socket_path')
            or os.environ.get('REMOTE_SOCKET_PATH')
            or config['default'].get('remote_socket_path')
            or '/run/podman/io.projectatomic.podman'
        )   # yapf:disable

        reqattr(
            'log_level',
            getattr(args, 'log_level')
            or os.environ.get('LOG_LEVEL')
            or config['default'].get('log_level')
            or logging.WARNING
        )  # yapf:disable

        setattr(
            args,
            'identity_file',
            getattr(args, 'identity_file')
            or os.environ.get('IDENTITY_FILE')
            or config['default'].get('identity_file')
            or os.path.expanduser('~{}/.ssh/id_dsa'.format(args.user))
        )   # yapf:disable

        if not os.path.isfile(args.identity_file):
            args.identity_file = None

        if args.host:
            args.local_socket_path = os.path.join(args.run_dir,
                                                  "podman.socket")
        else:
            args.local_socket_path = args.remote_socket_path

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
