![PODMAN logo](https://cdn.rawgit.com/kubernetes-incubator/cri-o/master/logo/crio-logo.svg)
# libpod - library for running OCI-based containers in Pods

## Podman Commands
| Command                                                  | Description                                                               | Demo|
| :------------------------------------------------------- | :------------------------------------------------------------------------ | :----|
| [podman(1)](/docs/podman.1.md)                           | Simple management tool for pods and images                                ||
| [podman-attach(1)](/docs/podman-attach.1.md)             | Attach to a running container                                             |[![...](/docs/play.png)](https://asciinema.org/a/XDlocUrHVETFECg4zlO9nBbLf)|
| [podman-build(1)](/docs/podman-build.1.md)               | Build an image using instructions from Dockerfiles                        ||
| [podman-commit(1)](/docs/podman-commit.1.md)             | Create new image based on the changed container                           ||
| [podman-cp(1)](/docs/podman-cp.1.md)                     | Instead of providing a `podman cp` command, the man page `podman-cp` describes how to use the `podman mount` command to have even more flexibility and functionality||
| [podman-create(1)](/docs/podman-create.1.md)             | Create a new container                                                    ||
| [podman-diff(1)](/docs/podman-diff.1.md)                 | Inspect changes on a container or image's filesystem                      |[![...](/docs/play.png)](https://asciinema.org/a/FXfWB9CKYFwYM4EfqW3NSZy1G)|
| [podman-exec(1)](/docs/podman-exec.1.md)                 | Execute a command in a running container
| [podman-export(1)](/docs/podman-export.1.md)             | Export container's filesystem contents as a tar archive                   |[![...](/docs/play.png)](https://asciinema.org/a/913lBIRAg5hK8asyIhhkQVLtV)|
| [podman-history(1)](/docs/podman-history.1.md)           | Shows the history of an image                                             |[![...](/docs/play.png)](https://asciinema.org/a/bCvUQJ6DkxInMELZdc5DinNSx)|
| [podman-images(1)](/docs/podman-images.1.md)             | List images in local storage                                              |[![...](/docs/play.png)](https://asciinema.org/a/133649)|
| [podman-import(1)](/docs/podman-import.1.md)             | Import a tarball and save it as a filesystem image                        ||
| [podman-info(1)](/docs/podman-info.1.md)                 | Display system information                                                |[![...](/docs/play.png)](https://asciinema.org/a/yKbi5fQ89y5TJ8e1RfJd4ivTD)|
| [podman-inspect(1)](/docs/podman-inspect.1.md)           | Display the configuration of a container or image                         |[![...](/docs/play.png)](https://asciinema.org/a/133418)|
| [podman-kill(1)](/docs/podman-kill.1.md)                 | Kill the main process in one or more running containers                   |[![...](/docs/play.png)](https://asciinema.org/a/3jNos0A5yzO4hChu7ddKkUPw7)|
| [podman-load(1)](/docs/podman-load.1.md)                 | Load an image from docker archive or oci                                  |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [podman-login(1)](/docs/podman-login.1.md)               | Login to a container registry						   |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
| [podman-logout(1)](/docs/podman-logout.1.md)             | Logout of a container registry                                            |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
| [podman-logs(1)](/docs/podman-logs.1.md)                 | Display the logs of a container                                           |[![...](/docs/play.png)](https://asciinema.org/a/MZPTWD5CVs3dMREkBxQBY9C5z)|
| [podman-mount(1)](/docs/podman-mount.1.md)               | Mount a working container's root filesystem                               |[![...](/docs/play.png)](https://asciinema.org/a/YSP6hNvZo0RGeMHDA97PhPAf3)|
| [podman-pause(1)](/docs/podman-pause.1.md)               | Pause one or more running containers                                      |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [podman-port(1)](/docs/podman-port.1.md)               | List port mappings for running containers |[![...](/docs/play.png)]()|
| [podman-ps(1)](/docs/podman-ps.1.md)                     | Prints out information about containers                                   |[![...](/docs/play.png)](https://asciinema.org/a/bbT41kac6CwZ5giESmZLIaTLR)|
| [podman-pull(1)](/docs/podman-pull.1.md)                 | Pull an image from a registry                                             |[![...](/docs/play.png)](https://asciinema.org/a/lr4zfoynHJOUNu1KaXa1dwG2X)|
| [podman-push(1)](/docs/podman-push.1.md)                 | Push an image to a specified destination                                  |[![...](/docs/play.png)](https://asciinema.org/a/133276)|
| [podman-restart](/docs/podman-restart.1.md)              | Restarts one or more containers                                           |[![...](/docs/play.png)](https://asciinema.org/a/jiqxJAxcVXw604xdzMLTkQvHM)|
| [podman-rm(1)](/docs/podman-rm.1.md)                     | Removes one or more containers                                            |[![...](/docs/play.png)](https://asciinema.org/a/7EMk22WrfGtKWmgHJX9Nze1Qp)|
| [podman-rmi(1)](/docs/podman-rmi.1.md)                   | Removes one or more images                                                |[![...](/docs/play.png)](https://asciinema.org/a/133799)|
| [podman-run(1)](/docs/podman-run.1.md)                   | Run a command in a container                                              ||
| [podman-save(1)](/docs/podman-save.1.md)                 | Saves an image to an archive                                              |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [podman-search(1)](/docs/podman-search.1.md)             | Search a registry for an image                                            ||
| [podman-start(1)](/docs/podman-start.1.md)               | Starts one or more containers
| [podman-stats(1)](/docs/podman-stats.1.md)               | Display a live stream of one or more containers' resource usage statistics|[![...](/docs/play.png)](https://asciinema.org/a/vfUPbAA5tsNWhsfB9p25T6xdr)|
| [podman-stop(1)](/docs/podman-stop.1.md)                 | Stops one or more running containers                                      |[![...](/docs/play.png)](https://asciinema.org/a/KNRF9xVXeaeNTNjBQVogvZBcp)|
| [podman-tag(1)](/docs/podman-tag.1.md)                   | Add an additional name to a local image                                   |[![...](/docs/play.png)](https://asciinema.org/a/133803)|
| [podman-top(1)](/docs/podman-top.1.md)                   | Display the running processes of a container              |[![...](/docs/play.png)](https://asciinema.org/a/5WCCi1LXwSuRbvaO9cBUYf3fk)|
| [podman-umount(1)](/docs/podman-umount.1.md)             | Unmount a working container's root filesystem                             |[![...](/docs/play.png)](https://asciinema.org/a/MZPTWD5CVs3dMREkBxQBY9C5z)|
| [podman-unpause(1)](/docs/podman-unpause.1.md)           | Unpause one or more running containers                                    |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [podman-varlink(1)](/docs/podman-varlink.1.md)           | Run the varlink backend                                           ||
| [podman-version(1)](/docs/podman-version.1.md)           | Display the version information                                           |[![...](/docs/play.png)](https://asciinema.org/a/mfrn61pjZT9Fc8L4NbfdSqfgu)|
| [podman-wait(1)](/docs/podman-wait.1.md)                 | Wait on one or more containers to stop and print their exit codes  |[![...](/docs/play.png)](https://asciinema.org/a/QNPGKdjWuPgI96GcfkycQtah0)|
