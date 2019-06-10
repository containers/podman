#!/usr/bin/env python3

# Simple and dumb script to send a message to the #podman IRC channel on frenode
# Based on example from: https://pythonspot.com/building-an-irc-bot/

import os
import time
import random
import errno
import socket
import sys

class IRC:

    response_timeout = 10  # seconds
    irc = socket.socket()

    def __init__(self, server, nickname, channel):
        self.server = server
        self.nickname = nickname
        self.channel = channel
        self.irc = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

    def _send(self, cmdstr):
        self.irc.send(bytes(cmdstr + '\r\n', 'utf-8'))

    def message(self, msg):
        data = 'PRIVMSG {0} :{1}\r\n'.format(self.channel, msg)
        print(data)
        self._send(data)

    @staticmethod
    def fix_newlines(bufr):
        return bufr.replace('\\r\\n', '\n')

    def _required_response(self, needle, haystack):
        start = time.time()
        end = start + self.response_timeout
        while time.time() < end:
            if haystack.find(needle) != -1:
                return (False, haystack)
            time.sleep(0.1)
            try:
                haystack += str(self.irc.recv(4096, socket.MSG_DONTWAIT))
            except socket.error as serr:
                if serr.errno == errno.EWOULDBLOCK:
                    continue
                raise  # can't handle this
        return (True, haystack)  # Error

    def connect(self, username, password):
        # This is ugly as sin, but seems to be a working send/expect sequence

        print("connecting to: {0}".format(self.server))
        self.irc.connect((self.server, 6667))  #connects to the server
        self._send("USER {0} {0} {0} :I am {0}".format(self.nickname))
        self._send("NICK {0}".format(self.nickname))

        err, haystack = self._required_response('End of /MOTD command.'
                                                ''.format(self.nickname), "")
        if err:
            print(self.fix_newlines(haystack))
            print("Error connecting to {0}".format(self.server))
            return True

        print("Logging in as {0}".format(username))
        self._send("PRIVMSG NickServ :IDENTIFY {0} {1}".format(username, password))
        err, _ = self._required_response("You are now identified for", "")
        if err:
            print("Error logging in to {0} as {1}".format(self.server, username))
            return True

        print("Joining {0}".format(self.channel))
        self._send("JOIN {0}".format(self.channel))
        err, haystack = self._required_response("{0} {1} :End of /NAMES list."
                                                "".format(self.nickname, self.channel),
                                                haystack)
        print(self.fix_newlines(haystack))
        if err:
            print("Error joining {0}".format(self.channel))
            return True
        return False

    def quit(self):
        print("Quitting")
        self._send("QUIT :my work is done here")
        self.irc.close()


if len(sys.argv) < 3:
    print("Error: Must pass desired nick and message as parameters")
else:
    irc = IRC("irc.freenode.net", sys.argv[1], "#podman")
    err = irc.connect(*os.environ.get('IRCID', 'Big Bug').split(" ", 2))
    if not err:
        irc.message(" ".join(sys.argv[2:]))
        time.sleep(5.0)  # avoid join/quit spam
        irc.quit()
