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

int rename_noreplace (int olddirfd, const char *oldpath, int newdirfd, const char *newpath)
{
  int ret;

# ifdef SYS_renameat2
#  ifndef RENAME_NOREPLACE
#   define RENAME_NOREPLACE	(1 << 0)
#  endif

  ret = (int) syscall (SYS_renameat2, olddirfd, oldpath, newdirfd, newpath, RENAME_NOREPLACE);
  if (ret == 0 || errno != EINVAL)
    return ret;

  /* Fallback in case of errno==EINVAL.  */
# endif

  /* This might be an issue if another process is trying to read the file while it is empty.  */
  ret = open (newpath, O_EXCL|O_CREAT, 0700);
  if (ret < 0)
    return ret;
  close (ret);

  /* We are sure we created the file, let's overwrite it.  */
  return rename (oldpath, newpath);
}

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
static fd_set *open_files_set;
static uid_t rootless_uid_init;
static gid_t rootless_gid_init;
static bool do_socket_activation = false;
static char *saved_systemd_listen_fds;
static char *saved_systemd_listen_pid;
static char *saved_systemd_listen_fdnames;

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
get_cmd_line_args ()
{
  int fd;
  char *buffer;
  size_t allocated;
  size_t used = 0;
  int ret;
  int i, argc = 0;
  char **argv;

  fd = open ("/proc/self/cmdline", O_RDONLY);
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
          if (tmp == NULL)
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

  argv = get_cmd_line_args ();
  if (argv == NULL)
    return false;

  if (strstr (argv[0], "podman") == NULL)
    {
      free (argv[0]);
      free (argv);
      return false;
    }

  for (argc = 0; argv[argc]; argc++)
    {
      if (argc == 0 || argv[argc][0] == '-')
        continue;

      if (strcmp (argv[argc], "mount") == 0
          || strcmp (argv[argc], "search") == 0
          || (strcmp (argv[argc], "system") == 0 && argv[argc+1] && strcmp (argv[argc+1], "service") != 0))
        {
          ret = false;
          break;
        }

      if (argv[argc+1] != NULL && (strcmp (argv[argc], "container") == 0 ||
	   strcmp (argv[argc], "image") == 0) &&
	   strcmp (argv[argc+1], "mount") == 0)
        {
          ret = false;
          break;
        }
    }

  free (argv[0]);
  free (argv);
  return ret;
}

int
is_fd_inherited(int fd)
{
  if (open_files_set == NULL || fd > open_files_max_fd || fd < 0)
    return 0;

  return FD_ISSET(fd % FD_SETSIZE, &(open_files_set[fd / FD_SETSIZE])) ? 1 : 0;
}

static void __attribute__((constructor)) init()
{
  const char *xdg_runtime_dir;
  const char *pause;
  const char *listen_pid;
  const char *listen_fds;
  const char *listen_fdnames;

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
      size_t size = 0;

      for (ent = readdir (d); ent; ent = readdir (d))
        {
          int fd;

          if (ent->d_name[0] == '.')
            continue;

          fd = atoi (ent->d_name);
          if (fd == dirfd (d))
            continue;

          if (fd >= size * FD_SETSIZE)
            {
              int i;
              size_t new_size;

              new_size = (fd / FD_SETSIZE) + 1;
              open_files_set = realloc (open_files_set, new_size * sizeof (fd_set));
              if (open_files_set == NULL)
                _exit (EXIT_FAILURE);

              for (i = size; i < new_size; i++)
                FD_ZERO (&(open_files_set[i]));

              size = new_size;
            }

          if (fd > open_files_max_fd)
            open_files_max_fd = fd;

          FD_SET (fd % FD_SETSIZE, &(open_files_set[fd / FD_SETSIZE]));
        }
      closedir (d);
    }

    listen_pid = getenv("LISTEN_PID");
    listen_fds = getenv("LISTEN_FDS");
    listen_fdnames = getenv("LISTEN_FDNAMES");

    if (listen_pid != NULL && listen_fds != NULL && strtol(listen_pid, NULL, 10) == getpid())
      {
        // save systemd socket environment for rootless child
        do_socket_activation = true;
        saved_systemd_listen_pid = strdup(listen_pid);
        saved_systemd_listen_fds = strdup(listen_fds);
        saved_systemd_listen_fdnames = strdup(listen_fdnames);
        if (saved_systemd_listen_pid == NULL
                || saved_systemd_listen_fds == NULL
                || saved_systemd_listen_fdnames == NULL)
          {
            fprintf (stderr, "save socket listen environments error: %s\n", strerror (errno));
            _exit (EXIT_FAILURE);
          }
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
      const char *const suffix = "/libpod/tmp/pause.pid";
      char *cwd = getcwd (NULL, 0);
      char uid_fmt[16];
      char gid_fmt[16];
      size_t len;

      if (cwd == NULL)
        {
          fprintf (stderr, "error getting current working directory: %s\n", strerror (errno));
          _exit (EXIT_FAILURE);
        }

      len = snprintf (path, PATH_MAX, "%s%s", xdg_runtime_dir, suffix);
      if (len >= PATH_MAX)
        {
          fprintf (stderr, "invalid value for XDG_RUNTIME_DIR: %s", strerror (ENAMETOOLONG));
          exit (EXIT_FAILURE);
        }

      fd = open (path, O_RDONLY);
      if (fd < 0)
        {
          free (cwd);
          return;
        }

      r = TEMP_FAILURE_RETRY (read (fd, buf, sizeof (buf) - 1));
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

      sprintf (uid_fmt, "%d", uid);
      sprintf (gid_fmt, "%d", gid);

      setenv ("_CONTAINERS_USERNS_CONFIGURED", "init", 1);
      setenv ("_CONTAINERS_ROOTLESS_UID", uid_fmt, 1);
      setenv ("_CONTAINERS_ROOTLESS_GID", gid_fmt, 1);

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
          fprintf (stderr, "cannot chdir to %s: %s\n", cwd, strerror (errno));
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
          if (rename_noreplace (AT_FDCWD, tmp_file_path, AT_FDCWD, pause_pid_file_path) < 0)
            {
              unlink (tmp_file_path);
              kill (pid, SIGKILL);
              _exit (EXIT_FAILURE);
            }

          r = TEMP_FAILURE_RETRY (write (p[1], "0", 1));
          if (r < 0)
            {
              fprintf (stderr, "cannot write to pipe: %s\n", strerror (errno));
              _exit (EXIT_FAILURE);
            }
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

static int
open_namespace (int pid_to_join, const char *ns_file)
{
  char ns_path[PATH_MAX];
  int ret;

  ret = snprintf (ns_path, PATH_MAX, "/proc/%d/ns/%s", pid_to_join, ns_file);
  if (ret == PATH_MAX)
    {
      fprintf (stderr, "internal error: namespace path too long\n");
      return -1;
    }

  return open (ns_path, O_CLOEXEC | O_RDONLY);
}

static void
join_namespace_or_die (const char *name, int ns_fd)
{
  if (setns (ns_fd, 0) < 0)
    {
      fprintf (stderr, "cannot set %s namespace\n", name);
      _exit (EXIT_FAILURE);
    }
}

int
reexec_userns_join (int pid_to_join, char *pause_pid_file_path)
{
  char uid[16];
  char gid[16];
  char **argv;
  int pid;
  int mnt_ns = -1;
  int user_ns = -1;
  char *cwd = getcwd (NULL, 0);
  sigset_t sigset, oldsigset;

  if (cwd == NULL)
    {
      fprintf (stderr, "error getting current working directory: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  sprintf (uid, "%d", geteuid ());
  sprintf (gid, "%d", getegid ());

  argv = get_cmd_line_args ();
  if (argv == NULL)
    {
      fprintf (stderr, "cannot read argv: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  user_ns = open_namespace (pid_to_join, "user");
  if (user_ns < 0)
    return user_ns;
  mnt_ns = open_namespace (pid_to_join, "mnt");
  if (mnt_ns < 0)
    {
      close (user_ns);
      return mnt_ns;
    }

  pid = fork ();
  if (pid < 0)
    fprintf (stderr, "cannot fork: %s\n", strerror (errno));

  if (pid)
    {
      int f;

      /* We passed down these fds, close them.  */
      close (user_ns);
      close (mnt_ns);

      for (f = 3; f <= open_files_max_fd; f++)
        if (is_fd_inherited (f))
          close (f);
      if (do_socket_activation)
        {
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

  if (do_socket_activation)
    {
      char s[32];
      sprintf (s, "%d", getpid());
      setenv ("LISTEN_PID", s, true);
      setenv ("LISTEN_FDS", saved_systemd_listen_fds, true);
      setenv ("LISTEN_FDNAMES", saved_systemd_listen_fdnames, true);
    }

  setenv ("_CONTAINERS_USERNS_CONFIGURED", "init", 1);
  setenv ("_CONTAINERS_ROOTLESS_UID", uid, 1);
  setenv ("_CONTAINERS_ROOTLESS_GID", gid, 1);

  if (prctl (PR_SET_PDEATHSIG, SIGTERM, 0, 0, 0) < 0)
    {
      fprintf (stderr, "cannot prctl(PR_SET_PDEATHSIG): %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
    }

  join_namespace_or_die ("user", user_ns);
  join_namespace_or_die ("mnt", mnt_ns);
  close (user_ns);
  close (mnt_ns);

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
      fprintf (stderr, "cannot chdir to %s: %s\n", cwd, strerror (errno));
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
  char **argv;
  char uid[16];
  char gid[16];
  char *cwd = getcwd (NULL, 0);
  sigset_t sigset, oldsigset;

  if (cwd == NULL)
    {
      fprintf (stderr, "error getting current working directory: %s\n", strerror (errno));
      _exit (EXIT_FAILURE);
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

          num_fds = strtol (saved_systemd_listen_fds, NULL, 10);
          if (num_fds != LONG_MIN && num_fds != LONG_MAX)
            {
              int f;

              for (f = 3; f < num_fds + 3; f++)
                if (is_fd_inherited (f))
                  close (f);
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

  argv = get_cmd_line_args ();
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
      setenv ("LISTEN_FDS", saved_systemd_listen_fds, true);
      setenv ("LISTEN_FDNAMES", saved_systemd_listen_fdnames, true);
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
  if (ret != 1 || b != '0')
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
      fprintf (stderr, "cannot chdir to %s: %s\n", cwd, strerror (errno));
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
  if (ret < 0)
  {
	  fprintf (stderr, "cannot write to ready pipe: %s\n", strerror (errno));
	  _exit (EXIT_FAILURE);
  }
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
