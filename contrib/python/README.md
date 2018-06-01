# podman - pythonic library for working with varlink interface to Podman

## Status: Active Development

See [libpod](https://github.com/projectatomic/libpod)

## Releases

To build the podman egg:

```sh
cd ~/libpod/contrib/pypodman
python3 setup.py clean -a && python3 setup.py bdist
```

## Code snippets/examples:

### Show images in storage

```python
import podman

with podman.Client() as client:
  list(map(print, client.images.list()))
```

### Show containers created since midnight

```python
from datetime import datetime, time, timezone

import podman

midnight = datetime.combine(datetime.today(), time.min, tzinfo=timezone.utc)

with podman.Client() as client:
    for c in client.containers.list():
        created_at = podman.datetime_parse(c.createdat)

        if created_at > midnight:
            print('Container {}: image: {} created at: {}'.format(
                c.id[:12], c.image[:32], podman.datetime_format(created_at)))
```
