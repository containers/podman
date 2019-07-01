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
#include <stdio.h>

#ifndef RENAME_NOREPLACE
# define RENAME_NOREPLACE	(1 << 0)

int renameat2 (int olddirfd, const char *oldpath, int newdirfd, const char *newpath, unsigned int flags)
{
# ifdef __NR_renameat2
  return (int) syscall (__NR_renameat2, olddirfd, oldpath, newdirfd, newpath, flags);
# else
  /* no way to implement it atomically.  */
  errno = ENOSYS;
  return -1;
# endif
}
#endif

#ifndef TEMP_FAILURE_RETRY
#define TEMP_FAILURE_RETRY(expression) \
  (__extension__                                                              \
    ({ long int __result;                                                     \
       do __result = (long int) (expression);                                 \
       while (__result == -1L && errno == EINTR);                             \
       __result; }))
#endif

static const char *_max_user_namespaces = "/proc/sys/user/max_user_namespaces";
static const char *_unprivileged_user_namespaces = "/proc/sys/kernel/unprivileged_userns_clone";

static int open_files_max_fd;
fd_set open_files_set;
static uid_t rootless_uid_init;
static gid_t rootless_gid_init;

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

uid_t
rootless_uid ()
{
  return rootless_uid_init;
}

uid_t
rootless_gid ()
{
  return rootless_gid_init;
}

static void
do_pause ()
{
  int i;
  struct sigaction act;
  int const sig[] =
    {
     SIGALRM, SIGHUP, SIGINT, SIGPIPE, SIGQUIT, SIGPOLL,
     SIGPROF, SIGVTALRM, SIGXCPU, SIGXFSZ, 0
    };

  act.sa_handler = SIG_IGN;

  for (i = 0; sig[i]; i++)
    sigaction (sig[i], &act, NULL);

  prctl (PR_SET_NAME, "podman pause", NULL, NULL, NULL);
  while (1)
    pause ();
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

  if (pid)
    sprintf (path, "/proc/%d/cmdline", pid);
  else
    strcpy (path, "/proc/self/cmdline");
  fd = open (path, O_RDONLY);
  if (fd < 0)
    return NULL;

  allocated = 512;
  buffer = malloc (allocated);
  if (buffer == NULL)
    return NULL;
  for (;;)
    {
      ret = TEMP_FAILURE_RETRY (read (fd, buffer + used, allocated - used));
      if (ret < 0)
        {
          free (buffer);
          return NULL;
        }

      if (ret == 0)
        break;

      used += ret;
      if (allocated == used)
        {
          allocated += 512;
          char *tmp = realloc (buffer, allocated);
          if (buffer == NULL)
            {
              free (buffer);
              return NULL;
            }
	  buffer = tmp;
        }
    }
  close (fd);

  for (i = 0; i < used; i++)
    if (buffer[i] == '\0')
      argc++;
  if (argc == 0)
    {
      free (buffer);
      return NULL;
    }

  argv = malloc (sizeof (char *) * (argc + 1));
  if (argv == NULL)
    {
      free (buffer);
      return NULL;
    }
  argc = 0;

  argv[argc++] = buffer;
  for (i = 0; i < used - 1; i++)
    if (buffer[i] == '\0')
      argv[argc++] = buffer + i + 1;

  argv[argc] = NULL;

  return argv;
}

static bool
can_use_shortcut ()
{
  int argc;
  char **argv;
  bool ret = true;

#ifdef DISABLE_JOIN_SHORTCUT
  return false;
#endif

  argv = get_cmd_line_args (0);
  if (argv == NULL)
    return false;

  for (argc = 0; argv[argc]; argc++)
    {
      if (argc == 0 || argv[argc][0] == '-')
        continue;

      if (strcmp (argv[argc], "mount") == 0
          || strcmp (argv[argc], "search") == 0
          || strcmp (argv[argc], "system") == 0)
        {
          ret = false;
          break;
        }
    }

  free (argv[0]);
  free (argv);
  return ret;
}

static void __attribute__((constructor)) init()
{
  const char *xdg_runtime_dir;
  const char *pause;
  DIR *d;

  pause = getenv ("_PODMAN_PAUSE");
  if (pause && pause[0])
    {
      do_pause ();
      _exit (EXIT_FAILURE);
    }

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

  /* Shortcut.  If we are able to join the pause pid file, do it now so we don't
     need to re-exec.  */
  xdg_runtime_dir = getenv ("XDG_RUNTIME_DIR");
  if (geteuid () != 0 && xdg_runtime_dir && xdg_runtime_dir[0] && can_use_shortcut ())
    {
      int r;
      int fd;
      long pid;
      char buf[12];
      uid_t uid;
      gid_t gid;
      char path[PATH_MAX];
      const char *const suffix = "/libpod/pause.pid";
      char *cwd = getcwd (NULL, 0);

      if (cwd == NULL)
        {
          fprintf (stderr, "error getting current working directory: %s\n", strerror (errno));
          _exit (EXIT_FAILURE);
        }

      if (strlen (xdg_runtime_dir) >= PATH_MAX - strlen (suffix))
        {
          fprintf (stderr, "invalid value for XDG_RUNTIME_DIR: %s", strerror (ENAMETOOLONG));
          exit (EXIT_FAILURE);
        }

      sprintf (path, "%s%s", xdg_runtime_dir, suffix);
      fd = open (path, O_RDONLY);
      if (fd < 0)
        {
          free (cwd);
          return;
        }

      r = TEMP_FAILURE_RETRY (read (fd, buf, sizeof (buf)));
      close (fd);
      if (r < 0)
        {
          free (cwd);
          return;
        }
      buf[r] = '\0';

      pid = strtol (buf, NULL, 10);
      if (pid == LONG_MAX)
        {
          free (cwd);
          return;
        }

      uid = geteuid ();
      gid = getegid ();

      sprintf (path, "/proc/%ld/ns/user", pid);
      fd = open (path, O_RDONLY);
      if (fd < 0 || setns (fd, 0) < 0)
        {
          free (cwd);
          return;
        }
      close (fd);

      /* Errors here cannot be ignored as we already joined a ns.  */
      sprintf (path, "/proc/%ld/ns/mnt", pid);
      fd = open (path, O_RDONLY);
      if (fd < 0)
        {
          fprintf (stderr, "cannot open %s: %s", path, strerror (errno));
          exit (EXIT_FAILURE);
        }

      r = setns (fd, 0);
      if (r < 0)
        {
          fprintf (stderr, "cannot join mount namespace for %ld: %s", pid, strerror (errno));
          exit (EXIT_FAILURE);
        }
      close (fd);

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
      rootless_uid_init = uid;
      rootless_gid_init = gid;
    }
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

int
reexec_in_user_namespace_wait (int pid, int options)
{
  pid_t p;
  int status;

  p = TEMP_FAILURE_RETRY (waitpid (pid, &status, 0));
  if (p < 0)
    return -1;

  if (WIFEXITED (status))
    return WEXITSTATUS (status);
  if (WIFSIGNALED (status))
    return 128 + WTERMSIG (status);
  return -1;
}

static int
create_pause_process (const char *pause_pid_file_path, char **argv)
{
  int r, p[2];

  if (pipe (p) < 0)
    _exit (EXIT_FAILURE);

  r = fork ();
  if (r < 0)
    _exit (EXIT_FAILURE);

  if (r)
    {
      char b;

      close (p[1]);
      /* Block until we write the pid file.  */
      r = TEMP_FAILURE_RETRY (read (p[0], &b, 1));
      close (p[0]);

      reexec_in_user_namespace_wait (r, 0);

      return r == 1 && b == '0' ? 0 : -1;
    }
  else
    {
      int fd;
      pid_t pid;

      close (p[0]);

      setsid ();
      pid = fork ();
      if (r < 0)
        _exit (EXIT_FAILURE);

      if (pid)
        {
          char pid_str[12];
          char *tmp_file_path = NULL;

          sprintf (pid_str, "%d", pid);

          if (asprintf (&tmp_file_path, "%s.XXXXXX", pause_pid_file_path) < 0)
            {
              fprintf (stderr, "unable to print to string\n");
              kill (pid, SIGKILL);
              _exit (EXIT_FAILURE);
            }

          if (tmp_file_path == NULL)
            {
              fprintf (stderr, "temporary file path is NULL\n");
              kill (pid, SIGKILL);
              _exit (EXIT_FAILURE);
            }

          fd = mkstemp (tmp_file_path);
          if (fd < 0)
            {
              fprintf (stderr, "error creating temporary file: %s\n", strerror (errno));
              kill (pid, SIGKILL);
              _exit (EXIT_FAILURE);
            }

          r = TEMP_FAILURE_RETRY (write (fd, pid_str, strlen (pid_str)));
          if (r < 0)
            {
              fprintf (stderr, "cannot write to file descriptor: %s\n", strerror (errno));
              kill (pid, SIGKILL);
              _exit (EXIT_FAILURE);
            }
          close (fd);

          /* There can be another process at this point trying to configure the user namespace and the pause
           process, do not override the pid file if it already exists. */
          if (renameat2 (AT_FDCWD, tmp_file_path, AT_FDCWD, pause_pid_file_path, RENAME_NOREPLACE) < 0)
            {
              unlink (tmp_file_path);
              kill (pid, SIGKILL);
              _exit (EXIT_FAILURE);
            }

          r = TEMP_FAILURE_RETRY (write (p[1], "0", 1));
          close (p[1]);

          _exit (EXIT_SUCCESS);
        }
      else
        {
          int null;

          close (p[1]);

          null = open ("/dev/null", O_RDWR);
          if (null >= 0)
            {
              dup2 (null, 0);
              dup2 (null, 1);
              dup2 (null, 2);
              close (null);
            }

          for (fd = 3; fd < open_files_max_fd + 16; fd++)
            close (fd);

          setenv ("_PODMAN_PAUSE", "1", 1);
          execlp (argv[0], argv[0], NULL);

          /* If the execve fails, then do the pause here.  */
          do_pause ();
          _exit (EXIT_FAILURE);
        }
    }
}

int
reexec_userns_join (int userns, int mountns, char *pause_pid_file_path)
{
  pid_t ppid = getpid ();
  char uid[16];
  char gid[16];
  char **argv;
  int pid;
  char *cwd = getcwd (NULL, 0);
  sigset_t sigset, oldsigset;

  if (cwd == NULL)
    {
      fprintf (stderr, "error getting current working directory: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  sprintf (uid, "%d", geteuid ());
  sprintf (gid, "%d", getegid ());

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

  if (sigfillset (&sigset) < 0)
    {
      fprintf (stderr, "cannot fill sigset: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  if (sigdelset (&sigset, SIGCHLD) < 0)
    {
      fprintf (stderr, "cannot sigdelset(SIGCHLD): %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  if (sigdelset (&sigset, SIGTERM) < 0)
    {
      fprintf (stderr, "cannot sigdelset(SIGTERM): %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  if (sigprocmask (SIG_BLOCK, &sigset, &oldsigset) < 0)
    {
      fprintf (stderr, "cannot block signals: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  setenv ("_CONTAINERS_USERNS_CONFIGURED", "init", 1);
  setenv ("_CONTAINERS_ROOTLESS_UID", uid, 1);
  setenv ("_CONTAINERS_ROOTLESS_GID", gid, 1);

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
  close (mountns);

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

  if (pause_pid_file_path && pause_pid_file_path[0] != '\0')
    {
      /* We ignore errors here as we didn't create the namespace anyway.  */
      create_pause_process (pause_pid_file_path, argv);
    }
  if (sigprocmask (SIG_SETMASK, &oldsigset, NULL) < 0)
    {
      fprintf (stderr, "cannot block signals: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

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

static int
copy_file_to_fd (const char *file_to_read, int outfd)
{
  char buf[512];
  int fd;

  fd = open (file_to_read, O_RDONLY);
  if (fd < 0)
    return fd;

  for (;;)
    {
      ssize_t r, w, t = 0;

      r = TEMP_FAILURE_RETRY (read (fd, buf, sizeof buf));
      if (r < 0)
        {
          close (fd);
          return r;
        }

      if (r == 0)
        break;

      while (t < r)
        {
          w = TEMP_FAILURE_RETRY (write (outfd, &buf[t], r - t));
          if (w < 0)
            {
              close (fd);
              return w;
            }
          t += w;
        }
    }
  close (fd);
  return 0;
}

int
reexec_in_user_namespace (int ready, char *pause_pid_file_path, char *file_to_read, int outputfd)
{
  int ret;
  pid_t pid;
  char b;
  pid_t ppid = getpid ();
  char **argv;
  char uid[16];
  char gid[16];
  char *listen_fds = NULL;
  char *listen_pid = NULL;
  bool do_socket_activation = false;
  char *cwd = getcwd (NULL, 0);
  sigset_t sigset, oldsigset;

  if (cwd == NULL)
    {
      fprintf (stderr, "error getting current working directory: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  listen_pid = getenv("LISTEN_PID");
  listen_fds = getenv("LISTEN_FDS");

  if (listen_pid != NULL && listen_fds != NULL)
    {
      if (strtol(listen_pid, NULL, 10) == getpid())
        do_socket_activation = true;
    }

  sprintf (uid, "%d", geteuid ());
  sprintf (gid, "%d", getegid ());

  pid = syscall_clone (CLONE_NEWUSER|CLONE_NEWNS|SIGCHLD, NULL);
  if (pid < 0)
    {
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

  if (sigfillset (&sigset) < 0)
    {
      fprintf (stderr, "cannot fill sigset: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  if (sigdelset (&sigset, SIGCHLD) < 0)
    {
      fprintf (stderr, "cannot sigdelset(SIGCHLD): %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  if (sigdelset (&sigset, SIGTERM) < 0)
    {
      fprintf (stderr, "cannot sigdelset(SIGTERM): %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  if (sigprocmask (SIG_BLOCK, &sigset, &oldsigset) < 0)
    {
      fprintf (stderr, "cannot block signals: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  argv = get_cmd_line_args (ppid);
  if (argv == NULL)
    {
      fprintf (stderr, "cannot read argv: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  if (do_socket_activation)
    {
      char s[32];
      sprintf (s, "%d", getpid());
      setenv ("LISTEN_PID", s, true);
    }

  setenv ("_CONTAINERS_USERNS_CONFIGURED", "init", 1);
  setenv ("_CONTAINERS_ROOTLESS_UID", uid, 1);
  setenv ("_CONTAINERS_ROOTLESS_GID", gid, 1);

  ret = TEMP_FAILURE_RETRY (read (ready, &b, 1));
  if (ret < 0)
    {
      fprintf (stderr, "cannot read from sync pipe: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }
  if (b != '0')
    _exit (EXIT_FAILURE);

  if (syscall_setresgid (0, 0, 0) < 0)
    {
      fprintf (stderr, "cannot setresgid: %s\n", strerror (errno));
      TEMP_FAILURE_RETRY (write (ready, "1", 1));
      _exit (EXIT_FAILURE);
    }

  if (syscall_setresuid (0, 0, 0) < 0)
    {
      fprintf (stderr, "cannot setresuid: %s\n", strerror (errno));
      TEMP_FAILURE_RETRY (write (ready, "1", 1));
      _exit (EXIT_FAILURE);
    }

  if (chdir (cwd) < 0)
    {
      fprintf (stderr, "cannot chdir: %s\n", strerror (errno));
      TEMP_FAILURE_RETRY (write (ready, "1", 1));
      _exit (EXIT_FAILURE);
    }
  free (cwd);

  if (pause_pid_file_path && pause_pid_file_path[0] != '\0')
    {
      if (create_pause_process (pause_pid_file_path, argv) < 0)
        {
          TEMP_FAILURE_RETRY (write (ready, "2", 1));
          _exit (EXIT_FAILURE);
        }
    }

  ret = TEMP_FAILURE_RETRY (write (ready, "0", 1));
  close (ready);

  if (sigprocmask (SIG_SETMASK, &oldsigset, NULL) < 0)
    {
      fprintf (stderr, "cannot block signals: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  if (file_to_read && file_to_read[0])
    {
      ret = copy_file_to_fd (file_to_read, outputfd);
      close (outputfd);
      _exit (ret == 0 ? EXIT_SUCCESS : EXIT_FAILURE);
    }

  execvp (argv[0], argv);

  _exit (EXIT_FAILURE);
}
