% kpod(1) kpod-history - Simple tool to view the history of an image
% Urvashi Mohnani
% kpod-history "1" "JULY 2017" "kpod"

## NAME
kpod-history - Shows the history of an image

## SYNOPSIS
**kpod history**
**IMAGE[:TAG|DIGEST]**
[**--human**|**-H**]
[**--no-trunc**]
[**--quiet**|**-q**]
[**--format**]
[**--help**|**-h**]

## DESCRIPTION
**kpod history** displays the history of an image by printing out information
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
| .Created        | if **--human**, time elapsed since creation, otherwise time stamp of creation |
| .CreatedBy      | Command used to create the layer                                              |
| .Size           | Size of layer on disk                                                         |
| .Comment        | Comment for the layer                                                         |

**kpod [GLOBAL OPTIONS]**

**kpod history [GLOBAL OPTIONS]**

**kpod history [OPTIONS] IMAGE[:TAG|DIGEST]**

## OPTIONS

**--human, -H**
    Display sizes and dates in human readable format

**--no-trunc**
    Do not truncate the output

**--quiet, -q**
    Print the numeric IDs only

**--format**
    Alter the output for a format like 'json' or a Go template.


## EXAMPLES

```
# kpod history debian
ID              CREATED       CREATED BY                                      SIZE       COMMENT
b676ca55e4f2c   9 weeks ago   /bin/sh -c #(nop) CMD ["bash"]                  0 B
<missing>       9 weeks ago   /bin/sh -c #(nop) ADD file:ebba725fb97cea4...   45.14 MB
```

```
# kpod history --no-trunc=true --human=false debian
ID              CREATED                CREATED BY                                      SIZE       COMMENT
b676ca55e4f2c   2017-07-24T16:52:55Z   /bin/sh -c #(nop) CMD ["bash"]                  0
<missing>       2017-07-24T16:52:54Z   /bin/sh -c #(nop) ADD file:ebba725fb97cea4...   45142935
```

```
# kpod history --format "{{.ID}} {{.Created}}" debian
b676ca55e4f2c   9 weeks ago
<missing>       9 weeks ago
```

```
# kpod history --format json debian
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

## history
Show the history of an image

## SEE ALSO
kpod(1), crio(8), crio.conf(5)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
