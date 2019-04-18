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
#include <stdbool.h>
#include <sys/types.h>
#include <sys/prctl.h>
#include <dirent.h>
#include <sys/select.h>

static const char *_max_user_namespaces = "/proc/sys/user/max_user_namespaces";
static const char *_unprivileged_user_namespaces = "/proc/sys/kernel/unprivileged_userns_clone";

static int open_files_max_fd;
fd_set open_files_set;

static void __attribute__((constructor)) init()
{
  DIR *d;

  /* Store how many FDs were open before the Go runtime kicked in.  */
  d = opendir ("/proc/self/fd");
  if (d)
    {
      struct dirent *ent;

      FD_ZERO (&open_files_set);
      for (ent = readdir (d); ent; ent = readdir (d))
        {
          int fd = atoi (ent->d_name);
          if (fd != dirfd (d))
            {
              if (fd > open_files_max_fd)
                open_files_max_fd = fd;
              FD_SET (fd, &open_files_set);
            }
        }
      closedir (d);
    }
}


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
#if defined(__s390__) || defined(__CRIS__)
  return (int) syscall (__NR_clone, child_stack, flags);
#else
  return (int) syscall (__NR_clone, flags, child_stack);
#endif
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
          char *tmp = realloc (buffer, allocated);
          if (buffer == NULL) {
		  free(buffer);
		  return NULL;
	  }
	  buffer=tmp;
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
reexec_userns_join (int userns, int mountns)
{
  pid_t ppid = getpid ();
  char uid[16];
  char **argv;
  int pid;
  char *cwd = getcwd (NULL, 0);

  if (cwd == NULL)
    {
      fprintf (stderr, "error getting current working directory: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

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
    {
      /* We passed down these fds, close them.  */
      int f;
      for (f = 3; f < open_files_max_fd; f++)
        {
          if (FD_ISSET (f, &open_files_set))
            close (f);
        }
      return pid;
    }

  setenv ("_CONTAINERS_USERNS_CONFIGURED", "init", 1);
  setenv ("_CONTAINERS_ROOTLESS_UID", uid, 1);

  if (prctl (PR_SET_PDEATHSIG, SIGTERM, 0, 0, 0) < 0)
    {
      fprintf (stderr, "cannot prctl(PR_SET_PDEATHSIG): %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  if (setns (userns, 0) < 0)
    {
      fprintf (stderr, "cannot setns: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  close (userns);

  if (mountns >= 0 && setns (mountns, 0) < 0)
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

  if (chdir (cwd) < 0)
    {
      fprintf (stderr, "cannot chdir: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  free (cwd);

  execvp (argv[0], argv);

  _exit (EXIT_FAILURE);
}

static void
check_proc_sys_userns_file (const char *path)
{
  FILE *fp;
  fp = fopen (path, "r");
  if (fp)
    {
      char buf[32];
      size_t n_read = fread (buf, 1, sizeof(buf) - 1, fp);
      if (n_read > 0)
        {
          buf[n_read] = '\0';
          if (strtol (buf, NULL, 10) == 0)
            fprintf (stderr, "user namespaces are not enabled in %s\n", path);
        }
      fclose (fp);
    }
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
  char *listen_fds = NULL;
  char *listen_pid = NULL;
  bool do_socket_activation = false;
  char *cwd = getcwd (NULL, 0);

  if (cwd == NULL)
    {
      fprintf (stderr, "error getting current working directory: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  listen_pid = getenv("LISTEN_PID");
  listen_fds = getenv("LISTEN_FDS");

  if (listen_pid != NULL && listen_fds != NULL) {
    if (strtol(listen_pid, NULL, 10) == getpid()) {
      do_socket_activation = true;
    }
  }

  sprintf (uid, "%d", geteuid ());

  pid = syscall_clone (CLONE_NEWUSER|CLONE_NEWNS|SIGCHLD, NULL);
  if (pid < 0)
    {
      FILE *fp;
      fprintf (stderr, "cannot clone: %s\n", strerror (errno));
      check_proc_sys_userns_file (_max_user_namespaces);
      check_proc_sys_userns_file (_unprivileged_user_namespaces);
    }
  if (pid)
    {
      if (do_socket_activation)
        {
          long num_fds;
          num_fds = strtol (listen_fds, NULL, 10);
          if (num_fds != LONG_MIN && num_fds != LONG_MAX)
            {
              long i;
              for (i = 3; i < num_fds + 3; i++)
                if (FD_ISSET (i, &open_files_set))
                  close (i);
            }
          unsetenv ("LISTEN_PID");
          unsetenv ("LISTEN_FDS");
          unsetenv ("LISTEN_FDNAMES");
        }
      return pid;
    }

  argv = get_cmd_line_args (ppid);
  if (argv == NULL)
    {
      fprintf (stderr, "cannot read argv: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  if (do_socket_activation) {
    char s[32];
    sprintf (s, "%d", getpid());
    setenv ("LISTEN_PID", s, true);
  }

  setenv ("_CONTAINERS_USERNS_CONFIGURED", "init", 1);
  setenv ("_CONTAINERS_ROOTLESS_UID", uid, 1);

  do
    ret = read (ready, &b, 1) < 0;
  while (ret < 0 && errno == EINTR);
  if (ret < 0)
    {
      fprintf (stderr, "cannot read from sync pipe: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  close (ready);
  if (b != '1')
    _exit (EXIT_FAILURE);

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

  if (chdir (cwd) < 0)
    {
      fprintf (stderr, "cannot chdir: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  free (cwd);

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
