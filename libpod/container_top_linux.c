#define _GNU_SOURCE
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/mount.h>
#include <sys/wait.h>
#include <unistd.h>

/* keep special_exit_code in sync with container_top_linux.go */
int special_exit_code = 255;
char **argv = NULL;

void
create_argv (int len)
{
  /* allocate one extra element because we need a final NULL in c */
  argv = malloc (sizeof (char *) * (len + 1));
  if (argv == NULL)
    {
      fprintf (stderr, "failed to allocate ps argv");
      exit (special_exit_code);
    }
  /* add final NULL */
  argv[len] = NULL;
}

void
set_argv (int pos, char *arg)
{
  argv[pos] = arg;
}

/*
  We use cgo code here so we can fork then exec separately,
  this is done so we can mount proc after the fork because the pid namespace is
  only active after spawning childs.
*/
void
fork_exec_ps ()
{
  int r, status = 0;
  pid_t pid;

  if (argv == NULL)
    {
      fprintf (stderr, "argv not initialized");
      exit (special_exit_code);
    }

  pid = fork ();
  if (pid < 0)
    {
      fprintf (stderr, "fork: %m");
      exit (special_exit_code);
    }
  if (pid == 0)
    {
      r = mount ("proc", "/proc", "proc", 0, NULL);
      if (r < 0)
        {
          fprintf (stderr, "mount proc: %m");
          exit (special_exit_code);
        }
      /* use execve to unset all env vars, we do not want to leak anything into the container */
      execve (argv[0], argv, NULL);
      fprintf (stderr, "execve: %m");
      exit (special_exit_code);
    }

  r = waitpid (pid, &status, 0);
  if (r < 0)
    {
      fprintf (stderr, "waitpid: %m");
      exit (special_exit_code);
    }
  if (WIFEXITED (status))
    exit (WEXITSTATUS (status));
  if (WIFSIGNALED (status))
    exit (128 + WTERMSIG (status));
  exit (special_exit_code);
}
