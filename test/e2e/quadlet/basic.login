## assert-key-is Unit RequiresMountsFor "%t/containers"
## assert-key-is Service Type oneshot
## assert-key-is Service RemainAfterExit yes
## assert-key-is-regex Service ExecStart ".*/podman login foo.org"
## assert-key-is Service SyslogIdentifier "%N"

[Login]
Registry=foo.org
