## assert-podman-stop-post-args "logout"
## assert-podman-stop-post-args "--authfile" "/etc/certs/auth.json"
## assert-podman-stop-post-final-args "foo.org"

[Login]
Registry=foo.org
AuthFile=/etc/certs/auth.json
LogoutOnStop=yes
