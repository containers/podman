#define _GNU_SOURCE
#include <sched.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/syscall.h>
#include <stdlib.h>
#include <errno.h>
#include <sys/stat.h>
#include <limits.h>
#include <sys/types.h>
#include <signal.h>
#include <fcntl.h>
#include <sys/wait.h>
#include <string.h>

static int
syscall_setresuid (uid_t ruid, uid_t euid, uid_t suid)
{
  return (int) syscall (__NR_setresuid, ruid, euid, suid);
}

static int
syscall_setresgid (gid_t rgid, gid_t egid, gid_t sgid)
{
  return (int) syscall (__NR_setresgid, rgid, egid, sgid);
}

static int
syscall_clone (unsigned long flags, void *child_stack)
{
  return (int) syscall (__NR_clone, flags, child_stack);
}

static char **
get_cmd_line_args (pid_t pid)
{
  int fd;
  char path[PATH_MAX];
  char *buffer;
  size_t allocated;
  size_t used = 0;
  int ret;
  int i, argc = 0;
  char **argv;

  sprintf (path, "/proc/%d/cmdline", pid);
  fd = open (path, O_RDONLY);
  if (fd < 0)
    return NULL;

  allocated = 512;
  buffer = malloc (allocated);
  if (buffer == NULL)
    return NULL;
  for (;;)
    {
      do
        ret = read (fd, buffer + used, allocated - used);
      while (ret < 0 && errno == EINTR);
      if (ret < 0)
        return NULL;

      if (ret == 0)
        break;

      used += ret;
      if (allocated == used)
        {
          allocated += 512;
          buffer = realloc (buffer, allocated);
          if (buffer == NULL)
            return NULL;
        }
    }
  close (fd);

  for (i = 0; i < used; i++)
    if (buffer[i] == '\0')
      argc++;
  if (argc == 0)
    return NULL;

  argv = malloc (sizeof (char *) * (argc + 1));
  if (argv == NULL)
    return NULL;
  argc = 0;

  argv[argc++] = buffer;
  for (i = 0; i < used - 1; i++)
    if (buffer[i] == '\0')
      argv[argc++] = buffer + i + 1;

  argv[argc] = NULL;

  return argv;
}

int
reexec_userns_join (int userns)
{
  pid_t ppid = getpid ();
  char uid[16];
  char **argv;
  int pid;

  sprintf (uid, "%d", geteuid ());

  argv = get_cmd_line_args (ppid);
  if (argv == NULL)
    {
      fprintf (stderr, "cannot read argv: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  pid = fork ();
  if (pid < 0)
    fprintf (stderr, "cannot fork: %s\n", strerror (errno));
  if (pid)
    return pid;

  setenv ("_LIBPOD_USERNS_CONFIGURED", "init", 1);
  setenv ("_LIBPOD_ROOTLESS_UID", uid, 1);

  if (setns (userns, 0) < 0)
    {
      fprintf (stderr, "cannot setns: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  close (userns);

  if (syscall_setresgid (0, 0, 0) < 0)
    {
      fprintf (stderr, "cannot setresgid: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  if (syscall_setresuid (0, 0, 0) < 0)
    {
      fprintf (stderr, "cannot setresuid: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  execvp (argv[0], argv);

  _exit (EXIT_FAILURE);
}

int
reexec_in_user_namespace (int ready)
{
  int ret;
  pid_t pid;
  char b;
  pid_t ppid = getpid ();
  char **argv;
  char uid[16];

  sprintf (uid, "%d", geteuid ());

  pid = syscall_clone (CLONE_NEWUSER|CLONE_NEWNS|SIGCHLD, NULL);
  if (pid < 0)
    fprintf (stderr, "cannot clone: %s\n", strerror (errno));
  if (pid)
    return pid;

  argv = get_cmd_line_args (ppid);
  if (argv == NULL)
    {
      fprintf (stderr, "cannot read argv: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  setenv ("_LIBPOD_USERNS_CONFIGURED", "init", 1);
  setenv ("_LIBPOD_ROOTLESS_UID", uid, 1);

  do
    ret = read (ready, &b, 1) < 0;
  while (ret < 0 && errno == EINTR);
  if (ret < 0)
    {
      fprintf (stderr, "cannot read from sync pipe: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  close (ready);

  if (syscall_setresgid (0, 0, 0) < 0)
    {
      fprintf (stderr, "cannot setresgid: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  if (syscall_setresuid (0, 0, 0) < 0)
    {
      fprintf (stderr, "cannot setresuid: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  execvp (argv[0], argv);

  _exit (EXIT_FAILURE);
}

int
reexec_in_user_namespace_wait (int pid)
{
  pid_t p;
  int status;

  do
    p = waitpid (pid, &status, 0);
  while (p < 0 && errno == EINTR);

  if (p < 0)
    return -1;

  if (WIFEXITED (status))
    return WEXITSTATUS (status);
  if (WIFSIGNALED (status))
    return 128 + WTERMSIG (status);
  return -1;
}
