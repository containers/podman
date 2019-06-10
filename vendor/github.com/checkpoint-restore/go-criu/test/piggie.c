#define _GNU_SOURCE
#include <stdio.h>
#include <signal.h>
#include <unistd.h>
#include <sys/mman.h>
#include <fcntl.h>
#include <sched.h>

#define STKS	(4*4096)

#ifndef CLONE_NEWPID
#define CLONE_NEWPID    0x20000000
#endif

static int do_test(void *logf)
{
	int fd, i = 0;

	setsid();

	close(0);
	close(1);
	close(2);

	fd = open("/dev/null", O_RDONLY);
	if (fd != 0) {
		dup2(fd, 0);
		close(fd);
	}

	fd = open(logf, O_WRONLY | O_TRUNC | O_CREAT, 0600);
	dup2(fd, 1);
	dup2(fd, 2);
	if (fd != 1 && fd != 2)
		close(fd);

	while (1) {
		sleep(1);
		printf("%d\n", i++);
		fflush(stdout);
	}

	return 0;
}

int main(int argc, char **argv)
{
	int pid;
	void *stk;

	stk = mmap(NULL, STKS, PROT_READ | PROT_WRITE,
			MAP_PRIVATE | MAP_ANON | MAP_GROWSDOWN, 0, 0);
	pid = clone(do_test, stk + STKS, SIGCHLD | CLONE_NEWPID, argv[1]);
	printf("Child forked, pid %d\n", pid);

	return 0;
}
