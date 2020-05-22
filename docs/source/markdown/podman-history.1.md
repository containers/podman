% podman-history(1)

## NAME
podman\-history - Show the history of an image

## SYNOPSIS
**podman history** [*options*] *image*[:*tag*|@*digest*]

**podman image history** [*options*] *image*[:*tag*|@*digest*]

## DESCRIPTION
**podman history** displays the history of an image by printing out information
about each layer used in the image. The information printed out for each layer
include Created (time and date), Created By, Size, and Comment. The output can
be truncated or not using the **--no-trunc** flag. If the **--human** flag is
set, the time of creation and size are printed out in a human readable format.
The **--quiet** flag displays the ID of the image only when set and the **--format**
flag is used to print the information using the Go template provided by the user.

Valid placeholders for the Go template are listed below:

| **Placeholder** | **Description**                                                               |
| --------------- | ----------------------------------------------------------------------------- |
| .ID             | Image ID                                                                      |
| .Created        | if --human, time elapsed since creation, otherwise time stamp of creation     |
| .CreatedBy      | Command used to create the layer                                              |
| .Size           | Size of layer on disk                                                         |
| .Comment        | Comment for the layer                                                         |

## OPTIONS

**--human**, **-H**=*true|false*

Display sizes and dates in human readable format (default *true*).

**--no-trunc**=*true|false*

Do not truncate the output (default *false*).

**--notruncate**

Do not truncate the output

**--quiet**, **-q**=*true|false*

Print the numeric IDs only (default *false*).
**--format**=*format*

Alter the output for a format like 'json' or a Go template.

**--help**, **-h**

Print usage statement

## EXAMPLES

```
$ podman history debian
ID              CREATED       CREATED BY                                      SIZE       COMMENT
b676ca55e4f2c   9 weeks ago   /bin/sh -c #(nop) CMD ["bash"]                  0 B
<missing>       9 weeks ago   /bin/sh -c #(nop) ADD file:ebba725fb97cea4...   45.14 MB
```

```
$ podman history --no-trunc=true --human=false debian
ID              CREATED                CREATED BY                                      SIZE       COMMENT
b676ca55e4f2c   2017-07-24T16:52:55Z   /bin/sh -c #(nop) CMD ["bash"]                  0
<missing>       2017-07-24T16:52:54Z   /bin/sh -c #(nop) ADD file:ebba725fb97cea4...   45142935
```

```
$ podman history --format "{{.ID}} {{.Created}}" debian
b676ca55e4f2c   9 weeks ago
<missing>       9 weeks ago
```

```
$ podman history --format json debian
[
    {
	"id": "b676ca55e4f2c0ce53d0636438c5372d3efeb5ae99b676fa5a5d1581bad46060",
	"created": "2017-07-24T16:52:55.195062314Z",
	"createdBy": "/bin/sh -c #(nop)  CMD [\"bash\"]",
	"size": 0,
	"comment": ""
    },
    {
	"id": "b676ca55e4f2c0ce53d0636438c5372d3efeb5ae99b676fa5a5d1581bad46060",
	"created": "2017-07-24T16:52:54.898893387Z",
	"createdBy": "/bin/sh -c #(nop) ADD file:ebba725fb97cea45d0b1b35ccc8144e766fcfc9a78530465c23b0c4674b14042 in / ",
	"size": 45142935,
	"comment": ""
    }
]
```

## SEE ALSO
podman(1)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
