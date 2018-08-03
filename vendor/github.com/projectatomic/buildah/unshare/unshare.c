#define _GNU_SOURCE
#include <sys/types.h>
#include <sys/ioctl.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <grp.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <termios.h>
#include <unistd.h>

static int _buildah_unshare_parse_envint(const char *envname) {
	char *p, *q;
	long l;

	p = getenv(envname);
	if (p == NULL) {
		return -1;
	}
	q = NULL;
	l = strtol(p, &q, 10);
	if ((q == NULL) || (*q != '\0')) {
		fprintf(stderr, "Error parsing \"%s\"=\"%s\"!\n", envname, p);
		_exit(1);
	}
	unsetenv(envname);
	return l;
}

void _buildah_unshare(void)
{
	int flags, pidfd, continuefd, n, pgrp, sid, ctty, allow_setgroups;
	char buf[2048];

	flags = _buildah_unshare_parse_envint("_Buildah-unshare");
	if (flags == -1) {
		return;
	}
	if ((flags & CLONE_NEWUSER) != 0) {
		if (unshare(CLONE_NEWUSER) == -1) {
			fprintf(stderr, "Error during unshare(CLONE_NEWUSER): %m\n");
			_exit(1);
		}
	}
	pidfd = _buildah_unshare_parse_envint("_Buildah-pid-pipe");
	if (pidfd != -1) {
		snprintf(buf, sizeof(buf), "%llu", (unsigned long long) getpid());
		if (write(pidfd, buf, strlen(buf)) != strlen(buf)) {
			fprintf(stderr, "Error writing PID to pipe on fd %d: %m\n", pidfd);
			_exit(1);
		}
		close(pidfd);
	}
	continuefd = _buildah_unshare_parse_envint("_Buildah-continue-pipe");
	if (continuefd != -1) {
		n = read(continuefd, buf, sizeof(buf));
		if (n > 0) {
			fprintf(stderr, "Error: %.*s\n", n, buf);
			_exit(1);
		}
		close(continuefd);
	}
	sid = _buildah_unshare_parse_envint("_Buildah-setsid");
	if (sid == 1) {
		if (setsid() == -1) {
			fprintf(stderr, "Error during setsid: %m\n");
			_exit(1);
		}
	}
	pgrp = _buildah_unshare_parse_envint("_Buildah-setpgrp");
	if (pgrp == 1) {
		if (setpgrp() == -1) {
			fprintf(stderr, "Error during setpgrp: %m\n");
			_exit(1);
		}
	}
	ctty = _buildah_unshare_parse_envint("_Buildah-ctty");
	if (ctty != -1) {
		if (ioctl(ctty, TIOCSCTTY, 0) == -1) {
			fprintf(stderr, "Error while setting controlling terminal to %d: %m\n", ctty);
			_exit(1);
		}
	}
	allow_setgroups = _buildah_unshare_parse_envint("_Buildah-allow-setgroups");
	if ((flags & CLONE_NEWUSER) != 0) {
		if (allow_setgroups == 1) {
			if (setgroups(0, NULL) != 0) {
				fprintf(stderr, "Error during setgroups(0, NULL): %m\n");
				_exit(1);
			}
		}
		if (setresgid(0, 0, 0) != 0) {
			fprintf(stderr, "Error during setresgid(0): %m\n");
			_exit(1);
		}
		if (setresuid(0, 0, 0) != 0) {
			fprintf(stderr, "Error during setresuid(0): %m\n");
			_exit(1);
		}
	}
	if ((flags & ~CLONE_NEWUSER) != 0) {
		if (unshare(flags & ~CLONE_NEWUSER) == -1) {
			fprintf(stderr, "Error during unshare(...): %m\n");
			_exit(1);
		}
	}
	return;
}
