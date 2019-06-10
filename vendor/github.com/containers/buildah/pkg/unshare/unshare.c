#define _GNU_SOURCE
#include <sys/types.h>
#include <sys/ioctl.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <linux/memfd.h>
#include <fcntl.h>
#include <grp.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <termios.h>
#include <errno.h>
#include <unistd.h>

#ifndef F_LINUX_SPECIFIC_BASE
#define F_LINUX_SPECIFIC_BASE 1024
#endif
#ifndef F_ADD_SEALS
#define F_ADD_SEALS (F_LINUX_SPECIFIC_BASE + 9)
#define F_GET_SEALS (F_LINUX_SPECIFIC_BASE + 10)
#endif
#ifndef F_SEAL_SEAL
#define F_SEAL_SEAL   0x0001LU
#endif
#ifndef F_SEAL_SHRINK
#define F_SEAL_SHRINK 0x0002LU
#endif
#ifndef F_SEAL_GROW
#define F_SEAL_GROW   0x0004LU
#endif
#ifndef F_SEAL_WRITE
#define F_SEAL_WRITE  0x0008LU
#endif

#define BUFSTEP 1024

static const char *_max_user_namespaces = "/proc/sys/user/max_user_namespaces";
static const char *_unprivileged_user_namespaces = "/proc/sys/kernel/unprivileged_userns_clone";

static int _containers_unshare_parse_envint(const char *envname) {
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

static void _check_proc_sys_file(const char *path)
{
	FILE *fp;
	char buf[32];
	size_t n_read;
	long r;

	fp = fopen(path, "r");
	if (fp == NULL) {
		if (errno != ENOENT)
			fprintf(stderr, "Error reading %s: %m\n", _max_user_namespaces);
	} else {
		memset(buf, 0, sizeof(buf));
		n_read = fread(buf, 1, sizeof(buf) - 1, fp);
		if (n_read > 0) {
			r = atoi(buf);
			if (r == 0) {
				fprintf(stderr, "User namespaces are not enabled in %s.\n", path);
			}
		} else {
			fprintf(stderr, "Error reading %s: no contents, should contain a number greater than 0.\n", path);
		}
		fclose(fp);
	}
}

static char **parse_proc_stringlist(const char *list) {
	int fd, n, i, n_strings;
	char *buf, *new_buf, **ret;
	size_t size, new_size, used;

	fd = open(list, O_RDONLY);
	if (fd == -1) {
		return NULL;
	}
	buf = NULL;
	size = 0;
	used = 0;
	for (;;) {
		new_size = used + BUFSTEP;
		new_buf = realloc(buf, new_size);
		if (new_buf == NULL) {
			free(buf);
			fprintf(stderr, "realloc(%ld): out of memory\n", (long)(size + BUFSTEP));
			return NULL;
		}
		buf = new_buf;
		size = new_size;
		memset(buf + used, '\0', size - used);
		n = read(fd, buf + used, size - used - 1);
		if (n < 0) {
			fprintf(stderr, "read(): %m\n");
			return NULL;
		}
		if (n == 0) {
			break;
		}
		used += n;
	}
	close(fd);
	n_strings = 0;
	for (n = 0; n < used; n++) {
		if ((n == 0) || (buf[n-1] == '\0')) {
			n_strings++;
		}
	}
	ret = calloc(n_strings + 1, sizeof(char *));
	if (ret == NULL) {
		fprintf(stderr, "calloc(): out of memory\n");
		return NULL;
	}
	i = 0;
	for (n = 0; n < used; n++) {
		if ((n == 0) || (buf[n-1] == '\0')) {
			ret[i++] = &buf[n];
		}
	}
	ret[i] = NULL;
	return ret;
}

static int containers_reexec(void) {
	char **argv, *exename;
	int fd, mmfd, n_read, n_written;
	struct stat st;
	char buf[2048];

	argv = parse_proc_stringlist("/proc/self/cmdline");
	if (argv == NULL) {
		return -1;
	}
	fd = open("/proc/self/exe", O_RDONLY | O_CLOEXEC);
	if (fd == -1) {
		fprintf(stderr, "open(\"/proc/self/exe\"): %m\n");
		return -1;
	}
	if (fstat(fd, &st) == -1) {
		fprintf(stderr, "fstat(\"/proc/self/exe\"): %m\n");
		return -1;
	}
	exename = basename(argv[0]);
	mmfd = syscall(SYS_memfd_create, exename, (long) MFD_ALLOW_SEALING | MFD_CLOEXEC);
	if (mmfd == -1) {
		fprintf(stderr, "memfd_create(): %m\n");
		return -1;
	}
	for (;;) {
		n_read = read(fd, buf, sizeof(buf));
		if (n_read < 0) {
			fprintf(stderr, "read(\"/proc/self/exe\"): %m\n");
			return -1;
		}
		if (n_read == 0) {
			break;
		}
		n_written = write(mmfd, buf, n_read);
		if (n_written < 0) {
			fprintf(stderr, "write(anonfd): %m\n");
			return -1;
		}
		if (n_written != n_read) {
			fprintf(stderr, "write(anonfd): short write (%d != %d)\n", n_written, n_read);
			return -1;
		}
	}
	close(fd);
	if (fcntl(mmfd, F_ADD_SEALS, F_SEAL_SHRINK | F_SEAL_GROW | F_SEAL_WRITE | F_SEAL_SEAL) == -1) {
		close(mmfd);
		fprintf(stderr, "Error sealing memfd copy: %m\n");
		return -1;
	}
	if (fexecve(mmfd, argv, environ) == -1) {
		close(mmfd);
		fprintf(stderr, "Error during reexec(...): %m\n");
		return -1;
	}
	return 0;
}

void _containers_unshare(void)
{
	int flags, pidfd, continuefd, n, pgrp, sid, ctty;
	char buf[2048];

	flags = _containers_unshare_parse_envint("_Containers-unshare");
	if (flags == -1) {
		return;
	}
	if ((flags & CLONE_NEWUSER) != 0) {
		if (unshare(CLONE_NEWUSER) == -1) {
			fprintf(stderr, "Error during unshare(CLONE_NEWUSER): %m\n");
                        _check_proc_sys_file (_max_user_namespaces);
                        _check_proc_sys_file (_unprivileged_user_namespaces);
			_exit(1);
		}
	}
	pidfd = _containers_unshare_parse_envint("_Containers-pid-pipe");
	if (pidfd != -1) {
		snprintf(buf, sizeof(buf), "%llu", (unsigned long long) getpid());
		size_t size = write(pidfd, buf, strlen(buf));
		if (size != strlen(buf)) {
			fprintf(stderr, "Error writing PID to pipe on fd %d: %m\n", pidfd);
			_exit(1);
		}
		close(pidfd);
	}
	continuefd = _containers_unshare_parse_envint("_Containers-continue-pipe");
	if (continuefd != -1) {
		n = read(continuefd, buf, sizeof(buf));
		if (n > 0) {
			fprintf(stderr, "Error: %.*s\n", n, buf);
			_exit(1);
		}
		close(continuefd);
	}
	sid = _containers_unshare_parse_envint("_Containers-setsid");
	if (sid == 1) {
		if (setsid() == -1) {
			fprintf(stderr, "Error during setsid: %m\n");
			_exit(1);
		}
	}
	pgrp = _containers_unshare_parse_envint("_Containers-setpgrp");
	if (pgrp == 1) {
		if (setpgrp() == -1) {
			fprintf(stderr, "Error during setpgrp: %m\n");
			_exit(1);
		}
	}
	ctty = _containers_unshare_parse_envint("_Containers-ctty");
	if (ctty != -1) {
		if (ioctl(ctty, TIOCSCTTY, 0) == -1) {
			fprintf(stderr, "Error while setting controlling terminal to %d: %m\n", ctty);
			_exit(1);
		}
	}
	if ((flags & CLONE_NEWUSER) != 0) {
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
	if (containers_reexec() != 0) {
		_exit(1);
	}
	return;
}
