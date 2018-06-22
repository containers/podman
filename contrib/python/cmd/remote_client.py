import os
import getpass
import argparse
import images
import ps, rm, rmi
import sys
from utils import write_err
import pytoml

default_conf_path = "/etc/containers/podman_client.conf"

class HelpByDefaultArgumentParser(argparse.ArgumentParser):

    def error(self, message):
        write_err('%s: %s' % (self.prog, message))
        write_err("Try '%s --help' for more information." % self.prog)
        sys.exit(2)

    def print_usage(self, message="too few arguments"): # pylint: disable=arguments-differ
        self.prog = " ".join(sys.argv)
        self.error(message)


def create_parser(help_text):
    parser = HelpByDefaultArgumentParser(description=help_text)
    parser.add_argument('-v', '--version', action='version', version="0.0",
                        help=("show rpodman version and exit"))
    parser.add_argument('--debug', default=False, action='store_true',
                        help=("show debug messages"))
    parser.add_argument('--run_dir', dest="run_dir",
                        help=("directory to place socket bindings"))
    parser.add_argument('--user', dest="user",
                        help=("remote user"))
    parser.add_argument('--host', dest="host",
                        help=("remote host"))
    parser.add_argument('--remote_socket_path', dest="remote_socket_path",
                        help=("remote socket path"))
    parser.add_argument('--identity_file', dest="identity_file",
                        help=("path to identity file"))
    subparser = parser.add_subparsers(help=("commands"))
    images.cli(subparser)
    ps.cli(subparser)
    rm.cli(subparser)
    rmi.cli(subparser)

    return parser

def load_toml(path):
    # Lets load the configuration file
    with open(path) as stream:
        return pytoml.load(stream)

if __name__ == '__main__':

    host = None
    remote_socket_path = None
    user = None
    run_dir = None

    aparser = create_parser("podman remote tool")
    args = aparser.parse_args()
    if not os.path.exists(default_conf_path):
        conf = {"default": {}}
    else:
        conf = load_toml("/etc/containers/podman_client.conf")

    # run_dir
    if "run_dir" in os.environ:
        run_dir = os.environ["run_dir"]
    elif "run_dir" in conf["default"] and conf["default"]["run_dir"] is not None:
        run_dir = conf["default"]["run_dir"]
    else:
        xdg = os.environ["XDG_RUNTIME_DIR"]
        run_dir = os.path.join(xdg, "podman")

    # make the run_dir if it doesnt exist
    if not os.path.exists(run_dir):
       os.makedirs(run_dir)

    local_socket_path = os.path.join(run_dir, "podman.socket")

    # remote host
    if "host" in os.environ:
        host = os.environ["host"]
    elif getattr(args, "host") is not None:
        host = getattr(args, "host")
    else:
        host = conf["default"]["host"] if "host" in conf["default"] else None

    # remote user
    if "user" in os.environ:
        user = os.environ["user"]
    elif getattr(args, "user") is not None:
        user = getattr(args, "user")
    elif "user" in conf["default"] and conf["default"]["user"] is not None:
        user = conf["default"]["user"]
    else:
        user = getpass.getuser()

    # remote path
    if "remote_socket_path" in os.environ:
        remote_socket_path = os.environ["remote_socket_path"]
    elif getattr(args, "remote_socket_path") is not None:
        remote_socket_path = getattr(args, "remote_socket_path")
    elif "remote_socket_path" in conf["default"] and conf["default"]["remote_socket_path"]:
        remote_socket_path = conf["default"]["remote_socket_path"]
    else:
        remote_socket_path = None


    # identity file
    if "identity_file" in os.environ:
        identity_file = os.environ["identity_file"]
    elif getattr(args, "identity_file") is not None:
        identity_file = getattr(args, "identity_file")
    elif "identity_file" in conf["default"] and conf["default"]["identity_file"] is not None:
        identity_file = conf["default"]["identity_file"]
    else:
        identity_file = None

    if None in [host, local_socket_path, user, remote_socket_path]:
        print("missing input for local_socket, user, host, or remote_socket_path")
        sys.exit(1)

    local_uri = "unix:{}".format(local_socket_path)
    remote_uri = "ssh://{}@{}{}".format(user, host, remote_socket_path)

    _class = args._class() # pylint: disable=protected-access
    _class.set_args(args, local_uri, remote_uri, identity_file)

    if "func" in args:
        _func = getattr(_class, args.func)
        sys.exit(_func())
    else:
        aparser.print_usage()
        sys.exit(1)