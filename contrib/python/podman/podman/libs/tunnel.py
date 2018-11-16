"""Cache for SSH tunnels."""
import collections
import getpass
import logging
import os
import subprocess
import threading
import time
import weakref
from contextlib import suppress

import psutil

Context = collections.namedtuple('Context', (
    'uri',
    'interface',
    'local_socket',
    'remote_socket',
    'username',
    'hostname',
    'port',
    'identity_file',
    'ignore_hosts',
    'known_hosts',
))
Context.__new__.__defaults__ = (None, ) * len(Context._fields)


class Portal(collections.MutableMapping):
    """Expiring container for tunnels."""

    def __init__(self, sweap=25):
        """Construct portal, reap tunnels every sweap seconds."""
        self.data = collections.OrderedDict()
        self.sweap = sweap
        self.ttl = sweap * 2
        self.lock = threading.RLock()
        self._schedule_reaper()

    def __getitem__(self, key):
        """Given uri return tunnel and update TTL."""
        with self.lock:
            value, _ = self.data[key]
            self.data[key] = (value, time.time() + self.ttl)
            self.data.move_to_end(key)
            return value

    def __setitem__(self, key, value):
        """Store given tunnel keyed with uri."""
        if not isinstance(value, Tunnel):
            raise ValueError('Portals only support Tunnels.')

        with self.lock:
            self.data[key] = (value, time.time() + self.ttl)
            self.data.move_to_end(key)

    def __delitem__(self, key):
        """Remove and close tunnel from portal."""
        with self.lock:
            value, _ = self.data[key]
            del self.data[key]
            value.close()
            del value

    def __iter__(self):
        """Iterate tunnels."""
        with self.lock:
            values = self.data.values()

        for tunnel, _ in values:
            yield tunnel

    def __len__(self):
        """Return number of tunnels in portal."""
        with self.lock:
            return len(self.data)

    def _schedule_reaper(self):
        timer = threading.Timer(interval=self.sweap, function=self.reap)
        timer.setName('PortalReaper')
        timer.setDaemon(True)
        timer.start()

    def reap(self):
        """Remove tunnels who's TTL has expired."""
        now = time.time()
        with self.lock:
            reaped_data = self.data.copy()
            for entry in reaped_data.items():
                if entry[1][1] < now:
                    del self.data[entry[0]]
                else:
                    # StopIteration as soon as possible
                    break
            self._schedule_reaper()


class Tunnel():
    """SSH tunnel."""

    def __init__(self, context):
        """Construct Tunnel."""
        self.context = context
        self._closed = True

    @property
    def closed(self):
        """Is tunnel closed."""
        return self._closed

    def bore(self):
        """Create SSH tunnel from given context."""
        cmd = ['ssh', '-fNT']

        if logging.getLogger().getEffectiveLevel() == logging.DEBUG:
            cmd.append('-v')
        else:
            cmd.append('-q')

        if self.context.port:
            cmd.extend(('-p', str(self.context.port)))

        cmd.extend(('-L', '{}:{}'.format(self.context.local_socket,
                                         self.context.remote_socket)))

        if self.context.ignore_hosts:
            cmd.extend(('-o', 'StrictHostKeyChecking=no',
                        '-o', 'UserKnownHostsFile=/dev/null'))
        elif self.context.known_hosts:
            cmd.extend(('-o', 'UserKnownHostsFile=%s' % self.context.known_hosts))

        if self.context.identity_file:
            cmd.extend(('-i', self.context.identity_file))

        cmd.append('{}@{}'.format(self.context.username,
                                  self.context.hostname))

        logging.debug('Opening tunnel "%s", cmd "%s"', self.context.uri,
                      ' '.join(cmd))

        tunnel = subprocess.Popen(cmd, close_fds=True)
        # The return value of Popen() has no long term value as that process
        # has already exited by the time control is returned here. This is a
        # side effect of the -f option. wait() will be called to clean up
        # resources.
        for _ in range(300):
            # TODO: Make timeout configurable
            if os.path.exists(self.context.local_socket) \
                    or tunnel.returncode is not None:
                break
            with suppress(subprocess.TimeoutExpired):
                # waiting for either socket to be created
                # or first child to exit
                tunnel.wait(0.5)
        else:
            raise TimeoutError(
                'Failed to create tunnel "{}", using: "{}"'.format(
                    self.context.uri, ' '.join(cmd)))
        if tunnel.returncode is not None and tunnel.returncode != 0:
            raise subprocess.CalledProcessError(tunnel.returncode,
                                                ' '.join(cmd))
        tunnel.wait()

        self._closed = False
        weakref.finalize(self, self.close)
        return self

    def close(self):
        """Close SSH tunnel."""
        logging.debug('Closing tunnel "%s"', self.context.uri)

        if self._closed:
            return

        # Find all ssh instances for user with uri tunnel the hard way...
        targets = [
            p
            for p in psutil.process_iter(attrs=['name', 'username', 'cmdline'])
            if p.info['username'] == getpass.getuser()
            and p.info['name'] == 'ssh'
            and self.context.local_socket in ' '.join(p.info['cmdline'])
        ]  # yapf: disable

        # ask nicely for ssh to quit, reap results
        for proc in targets:
            proc.terminate()
        _, alive = psutil.wait_procs(targets, timeout=300)

        # kill off the uncooperative, then report any stragglers
        for proc in alive:
            proc.kill()
        _, alive = psutil.wait_procs(targets, timeout=300)

        for proc in alive:
            logging.info('process %d survived SIGKILL, giving up.', proc.pid)

        with suppress(OSError):
            os.remove(self.context.local_socket)
        self._closed = True
