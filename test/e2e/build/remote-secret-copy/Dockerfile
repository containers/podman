FROM alpine
COPY . .
RUN test -e hello
RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret
RUN sh -c 'test -z "$(find / -name '\''podman-build-secret*'\'' 2>/dev/null | head -n1)"'
