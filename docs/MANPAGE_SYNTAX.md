% podman-command(1)

## NAME
podman\-command - short description

## SYNOPSIS
(Shows the command structure.)

**podman command** [*optional*] *mandatory value*

**podman subcommand command** [*optional*] *mandatory value*

(If there is the possibility to chose between 2 (two) or more mandatory command values. There should also always be a space before and after a vertical bar to ensure better readability.)

**podman command** [*optional*] *value1* | *value2*

**podman subcommand command** [*optional*] *value1* | *value2*

(If an optinal value follows a mandatory one.)

**podman command** [*optional*] *value1* | *value2* [*optional*]

**podman subcommand command** [*optional*] *value1* | *value2* [*optional*]

(If the command accepts an infinite number of values.)

**podman command** [*optional*] *value* [*value* ...]

**podman subcommand command** [*optional*] *value* [*value* ...]

## DESCRIPTION
**podman command** is always the beginning of the DESCRIPTION section. Putting the command as the first part of the DESCRIPTION ensures uniformity. All commands mentioned in a text retain their appearance and form.\
Example sentence: The command **podman command** is an example command.\
It should also be specified if the command can only be run as root. In addition, it should be described when a command or OPTION cannot be executed with the remote client. For a command, this should be done in the DESCRIPTION part. For the OPTIONS, it should be done in the DESCRIPTION of the specified OPTION. Do not use pronouns in the man pages, especially the word `you`.

## OPTIONS
All flags are referred to as OPTIONS. The term flags should not be used. All OPTIONS are listed in this section. OPTIONS that appear in descriptions of other OPTIONS and sections retain their appearance, for example: **--exit**. Each OPTION should be explained to the fullest extend below the OPTION itself. Each OPTION is behind an H4-header (`####`).

#### **--option**, **-o**

OPTIONS can be put after the command in two different ways. Eather the long version with **--option** or as the short version **-o**. If there are two ways to write an OPTION they are separated by a comma. If there are 2 (two) versions of one command the long version is always shown in front.

#### **--exit**

An example of an OPTION that has only one possible structure. Thus, it cannot be executed by the extension **-e**.

#### **--answer**=, **-a**=**_active_** | *disable*

OPTIONS that accept 2 possible arguments as inputs are shown above. If there is a default argument that is selected when no special input is made, it is shown in **_bold italics_**. It must always be ensured that the standard argument is in the first place after the OPTION. In this example, there are 2 (two) different versions to execute the command. Both versions of the OPTION have to be shown with the arguments behind them.

#### **--status**=**good** | *better* | *best*

This is an example for 3 (three) arguments behind an OPTION. If the number of arguments is higher than 3 (three), the arguments are **not** listed after the equal sign. The arguments have to be explained in a table like in **--test**=**_test_** regardless of the number of arguments.

#### **--test**=**_test_**

OPTIONS that are followed by an equal sign include an argument after the equal sign in *italic*. If there is a default argument, that is used if the OPTION is not specified in the **command**, the argument after the eqaul sign is displayed in **bold**. All arguments must be listed and explained in the text below the OPTION.

| Argument           | Description                                                                 |
| -                  | -                                                                           |
| **_example one_**  | This argument is the standard argument if the OPTION is not specified.      |
| *example two*      | If one refers to a command, one should use **bold** marks.                  |
| *example three*    | Example: In combination with **podman command** highly effective.            |
| *example four*     | Example: Can be combined with **--exit**.                                   |
| *example five*     | The fifth description                                                       |

The table shows an example for a listing of arguments. The contents in the table should be aligned left. If the content in the table conflicts with this, it can be aligned in a way that supports the understanding of the content. If there is a standard argument, it **must** listed as the first entry in the table.

If the number of arguments is smaller than 4 (four) they have to be listed behind the OPTION as seen in the OPTION **--status**.

## SUBCHAPTER
For chapters that are made specifically as an individual SUBCHAPTER in a man page, the previous conditions regarding formatting apply.

There are no restrictions for the use of paragraphs and tables. Within these paragraphs and tables the previous conditions regarding formatting apply.

Strings of characters or numbers can be highlighted with `backticks`. Paths of any kind **must** be highlighted.\
IMPORTANT: Only characters that are **not** part of categories mentioned before can be highlighted. This includes headers. For example it is not advised to highlight an OPTION or a **command**.

SUBHEADINGS are displayed as follows:
### SUBHEADING
Text for SUBHEADINGS.

## EXAMPLES
All EXAMPLES are listed in this section. This section should be at the end of each man page. Each EXAMPLE is always in one box. The box starts and ends with the last written line, **not** with a blank line. The `$` in front of the commands indicates that it can be run as a normal user, while the commands starting with `#` can only be run as root.

### Description of the EXAMPLE
```
$ podman command

$ podman command -o

$ cat $HOME/Dockerfile | podman command --option
```

### Description of the EXAMPLE 2
```
$ podman command --redhat

$ podman command --redhat better

$ podman command --redhat=better
```
