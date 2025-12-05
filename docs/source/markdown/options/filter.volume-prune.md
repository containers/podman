#### **--filter**

Provide filter values.

The *filters* argument format is of `key=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| Filter      | Description                                                                                                |
|:-----------:|------------------------------------------------------------------------------------------------------------|
| dangling    | [Bool] Only remove volumes not referenced by any containers                                                |
| driver      | [String] Only remove volumes with the given driver                                                         |
| label       | [String] Only remove volumes, with (or without, in the case of label!=[...] is used) the specified labels. |
| name        | [String] Only remove volume with the given name                                                            |
| opt         | [String] Only remove volumes created with the given options                                                |
| scope       | [String] Only remove volumes with the given scope                                                          |
| until       | [DateTime] Only remove volumes created before given timestamp.                                             |
| after/since | [Volume] Filter by volumes created after the given VOLUME (name or tag)                                    |

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes volumes with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes volumes without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine's time.
