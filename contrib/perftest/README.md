## perftest : tool for benchmarking and profiling libpod library
perftest uses libpod as golang library and perform stress test and profile for CPU usage.

Build:

```
# cd $GOPATH/src/github.com/containers/libpod/contrib/perftest
# go build
# go install
```

Usage:

```
# perftest -h
Usage of perftest:

-count int
        count of loop counter for test (default 50)
-image string
        image-name to be used for test (default "docker.io/library/alpine:latest")

```

e.g.

```
# perftest
runc version spec: 1.0.1-dev
conmon version 1.12.0-dev, commit: b6c5cafeffa9b3cde89812207b29ccedd3102712

preparing test environment...
2018/11/05 16:52:14 profile: cpu profiling enabled, /tmp/profile626959338/cpu.pprof
Test Round: 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 47 48 49
Profile data

        Create  Start   Stop    Delete
Min     0.23s   0.34s   2.12s   0.51s
Avg     0.25s   0.38s   2.13s   0.54s
Max     0.27s   0.48s   2.13s   0.70s
Total   12.33s  18.82s  106.47s 26.91s
2018/11/05 16:54:59 profile: cpu profiling disabled, /tmp/profile626959338/cpu.pprof

```

Analyse CPU profile.

```
# go tool pprof -http=":8081" $GOPATH/src/github.com/containers/libpod/contrib/perftest/perftest /tmp/profile626959338/cpu.pprof
```
- Open http://localhost:8081 in webbrowser