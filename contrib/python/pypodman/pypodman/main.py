"""Remote podman client."""
import logging
import os
import sys
from subprocess import CalledProcessError

from pypodman.lib import PodmanArgumentParser


def main():
    """Entry point."""
    # Setup logging so we use stderr and can change logging level later
    # Do it now before there is any chance of a default setup hardcoding crap.
    log = logging.getLogger()
    fmt = logging.Formatter('%(asctime)s | %(levelname)-8s | %(message)s',
                            '%Y-%m-%d %H:%M:%S %Z')
    stderr = logging.StreamHandler(stream=sys.stderr)
    stderr.setFormatter(fmt)
    log.addHandler(stderr)
    log.setLevel(logging.WARNING)

    parser = PodmanArgumentParser()
    args = parser.parse_args()

    log.setLevel(args.log_level)
    logging.debug(
        'Logging initialized at level %s',
        logging.getLevelName(logging.getLogger().getEffectiveLevel()))

    def want_tb():
        """Add traceback when logging events."""
        return log.getEffectiveLevel() == logging.DEBUG

    try:
        if not os.path.exists(args.run_dir):
            os.makedirs(args.run_dir)
    except PermissionError as e:
        logging.critical(e, exc_info=want_tb())
        sys.exit(6)

    # class_(args).method() are set by the sub-command's parser
    returncode = None
    try:
        obj = args.class_(args)
    except AttributeError:
        parser.print_help(sys.stderr)
        sys.exit(1)
    except ValueError as e:
        print(e, file=sys.stderr, flush=True)
        sys.exit(1)
    except Exception as e:  # pylint: disable=broad-except
        logging.critical(repr(e), exc_info=want_tb())
        logging.warning('See subparser "%s" configuration.',
                        args.subparser_name)
        sys.exit(5)

    try:
        returncode = getattr(obj, args.method)()
    except KeyboardInterrupt:
        pass
    except AttributeError as e:
        logging.critical(e, exc_info=want_tb())
        logging.warning('See subparser "%s" configuration.',
                        args.subparser_name)
        returncode = 3
    except (
            CalledProcessError,
            ConnectionError,
            ConnectionRefusedError,
            ConnectionResetError,
            TimeoutError,
    ) as e:
        logging.critical(e, exc_info=want_tb())
        logging.info('Review connection arguments for correctness.')
        returncode = 4

    return 0 if returncode is None else returncode


if __name__ == '__main__':
    sys.exit(main())
