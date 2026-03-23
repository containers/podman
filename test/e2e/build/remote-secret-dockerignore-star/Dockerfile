FROM alpine
COPY . .
RUN test ! -e ignored-by-dockerignore.txt
RUN --mount=type=secret,id=MY_SECRET cat /run/secrets/MY_SECRET
RUN sh -c 'test -z "$(find / -name '\''podman-build-secret*'\'' 2>/dev/null | head -n1)"'
