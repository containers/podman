% podman-completion(1)

## NAME
podman\-completion - Generate shell completion scripts

## SYNOPSIS
**podman completion** [*options*] *bash*|*zsh*|*fish*|*powershell*

## DESCRIPTION
The completion command generates shell completion scripts for a variety of shells. Supported shells are **bash**, **zsh**, **fish** and **powershell**.

These script are used by the shell to provide suggestions and complete commands when you are typing the command and press [TAB].

Usually these scripts are automatically installed via the package manager.

## OPTIONS
#### **--file**, **-f**

Write the generated output to file.

#### **--no-desc**

Do not provide description in the completions.

## Installation

### BASH
Make sure you have `bash-completion` installed on the system.

To load the completion script into the current session run:
`source <(podman completion bash)`

To make it available for all bash sessions run:
`podman completion bash -f /etc/bash_completion.d/podman`


### ZSH
If shell completion is not already enabled in the environment you will need to enable it. You can execute the following once:
`echo "autoload -U compinit; compinit" >> ~/.zshrc`

To make it available for all zsh sessions run:
`podman completion zsh -f "${fpath[1]}/_podman"`

Once you reload the shell the auto-completion should be working.


### FISH
To load the completion script into the current session run:
`podman completion fish | source`

To make it available for all fish sessions run:
`podman completion fish -f ~/.config/fish/completions/podman.fish`

### POWERSHELL
To load the completion script into the current session run:
`podman.exe completion powershell | Out-String | Invoke-Expression`

To make it available in all powershell sessions that a user has, write the
completion output to a file and source that to the user's powershell profile.
More information about profiles is available with `Get-Help about_Profiles`.

## SEE ALSO
[podman(1)](podman.1.md)
