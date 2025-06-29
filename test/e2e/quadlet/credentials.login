## assert-podman-args "--username" "foo"
## !assert-podman-args "--password" "bar"
## assert-podman-args "--password-stdin"
## assert-key-is Service StandardInput "data"
## assert-key-is Service StandardInputText "bar"

[Login]
Registry=foo.org
Username=foo
Password=bar
