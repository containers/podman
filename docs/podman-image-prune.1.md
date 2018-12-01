% PODMAN(1) Podman Man Pages
% Brent Baude
% December 2018
# NAME
podman-image-prune - Remove all unused images

# SYNOPSIS
**podman image prune**
[**-h**|**--help**]

# DESCRIPTION
**podman image prune** removes all unused images from local storage. An unused image
is defined as an image that does not have any containers based on it.

## Examples ##

Remove all unused images from local storage
```
$ sudo podman image prune
f3e20dc537fb04cb51672a5cb6fdf2292e61d411315549391a0d1f64e4e3097e
324a7a3b2e0135f4226ffdd473e4099fd9e477a74230cdc35de69e84c0f9d907
6125002719feb1ddf3030acab1df6156da7ce0e78e571e9b6e9c250424d6220c
91e732da5657264c6f4641b8d0c4001c218ae6c1adb9dcef33ad00cafd37d8b6
e4e5109420323221f170627c138817770fb64832da7d8fe2babd863148287fca
77a57fa8285e9656dbb7b23d9efa837a106957409ddd702f995605af27a45ebe
```

## SEE ALSO
podman(1), podman-images

# HISTORY
December 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
