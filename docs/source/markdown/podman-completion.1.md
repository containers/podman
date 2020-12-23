% podman-completion(1)

## NAME
podman\-completion - Generate shell completion scripts

## SYNOPSIS
**podman completion** [*options*] *bash*|*zsh*|*fish*

## DESCRIPTION
The completion command allows you to generate shell completion scripts. Supported shells are **bash**, **zsh** and **fish**.

These script are used by the shell to provide suggestions and complete commands when you are typing the command and press [TAB].

Usually these scripts are automatically installed via the package manager.

## OPTIONS
#### **--file**, **-f**

Write the generated output to file.

#### **--no-desc**

Do not provide description in the completions.

## Installation

### BASH
Make sure you have `bash-completion` installed on your system.

To load the completion script into your current session run:
`source <(podman completion bash)`

To make it available in all your bash sessions run:
`podman completion bash -f /etc/bash_completion.d/podman`


### ZSH
If shell completion is not already enabled in your environment you will need to enable it. You can execute the following once:
`echo "autoload -U compinit; compinit" >> ~/.zshrc`

To make it available in all your zsh sessions run:
`podman completion zsh -f "${fpath[1]}/_podman"`

Once you reload the shell the autocompletion should be working.


### FISH
To load the completion script into your current session run:
`podman completion fish | source`

To make it available in all your fish sessions run:
`podman completion fish -f ~/.config/fish/completions/podman.fish`


## SEE ALSO
[podman(1)](podman.1.md)
