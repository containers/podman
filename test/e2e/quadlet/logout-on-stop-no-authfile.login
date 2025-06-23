## assert-podman-stop-post-args "logout"
## !assert-podman-stop-post-args "--authfile" "/etc/certs/auth.json"
## assert-podman-stop-post-final-args "foo.org"

[Login]
Registry=foo.org
LogoutOnStop=yes
