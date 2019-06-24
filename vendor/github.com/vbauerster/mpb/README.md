# Multi Progress Bar

[![GoDoc](https://godoc.org/github.com/vbauerster/mpb?status.svg)](https://godoc.org/github.com/vbauerster/mpb)
[![Build Status](https://travis-ci.org/vbauerster/mpb.svg?branch=master)](https://travis-ci.org/vbauerster/mpb)
[![Go Report Card](https://goreportcard.com/badge/github.com/vbauerster/mpb)](https://goreportcard.com/report/github.com/vbauerster/mpb)
[![codecov](https://codecov.io/gh/vbauerster/mpb/branch/master/graph/badge.svg)](https://codecov.io/gh/vbauerster/mpb)

**mpb** is a Go lib for rendering progress bars in terminal applications.

## Features

* __Multiple Bars__: Multiple progress bars are supported
* __Dynamic Total__: [Set total](https://github.com/vbauerster/mpb/issues/9#issuecomment-344448984) while bar is running
* __Dynamic Add/Remove__: Dynamically add or remove bars
* __Cancellation__: Cancel whole rendering process
* __Predefined Decorators__: Elapsed time, [ewma](https://github.com/VividCortex/ewma) based ETA, Percentage, Bytes counter
* __Decorator's width sync__:  Synchronized decorator's width among multiple bars

## Installation

```sh
go get github.com/vbauerster/mpb
```

_Note:_ it is preferable to go get from github.com, rather than gopkg.in. See issue [#11](https://github.com/vbauerster/mpb/issues/11).

## Usage

#### [Rendering single bar](examples/singleBar/main.go)
```go
    p := mpb.New(
        // override default (80) width
        mpb.WithWidth(64),
        // override default 120ms refresh rate
        mpb.WithRefreshRate(180*time.Millisecond),
    )

    total := 100
    name := "Single Bar:"
    // adding a single bar
    bar := p.AddBar(int64(total),
        // override default "[=>-]" style
        mpb.BarStyle("╢▌▌░╟"),
        mpb.PrependDecorators(
            // display our name with one space on the right
            decor.Name(name, decor.WC{W: len(name) + 1, C: decor.DidentRight}),
            // replace ETA decorator with "done" message, OnComplete event
            decor.OnComplete(
                // ETA decorator with ewma age of 60, and width reservation of 4
                decor.EwmaETA(decor.ET_STYLE_GO, 60, decor.WC{W: 4}), "done",
            ),
        ),
        mpb.AppendDecorators(decor.Percentage()),
    )
    // simulating some work
    max := 100 * time.Millisecond
    for i := 0; i < total; i++ {
        start := time.Now()
        time.Sleep(time.Duration(rand.Intn(10)+1) * max / 10)
        // ewma based decorators require work duration measurement
        bar.IncrBy(1, time.Since(start))
    }
    // wait for our bar to complete and flush
    p.Wait()
```

#### [Rendering multiple bars](examples/simple/main.go)
```go
    var wg sync.WaitGroup
    p := mpb.New(mpb.WithWaitGroup(&wg))
    total, numBars := 100, 3
    wg.Add(numBars)

    for i := 0; i < numBars; i++ {
        name := fmt.Sprintf("Bar#%d:", i)
        bar := p.AddBar(int64(total),
            mpb.PrependDecorators(
                // simple name decorator
                decor.Name(name),
                // decor.DSyncWidth bit enables column width synchronization
                decor.Percentage(decor.WCSyncSpace),
            ),
            mpb.AppendDecorators(
                // replace ETA decorator with "done" message, OnComplete event
                decor.OnComplete(
                    // ETA decorator with ewma age of 60
                    decor.EwmaETA(decor.ET_STYLE_GO, 60), "done",
                ),
            ),
        )
        // simulating some work
        go func() {
            defer wg.Done()
            max := 100 * time.Millisecond
            for i := 0; i < total; i++ {
                start := time.Now()
                time.Sleep(time.Duration(rand.Intn(10)+1) * max / 10)
                // ewma based decorators require work duration measurement
                bar.IncrBy(1, time.Since(start))
            }
        }()
    }
    // wait for all bars to complete and flush
    p.Wait()
```

#### [Dynamic total](examples/dynTotal/main.go)

![dynamic total](examples/gifs/godEMrCZmJkHYH1X9dN4Nm0U7.svg)

#### [Complex example](examples/complex/main.go)

![complex](examples/gifs/wHzf1M7sd7B3zVa2scBMnjqRf.svg)

#### [Bytes counters](examples/io/single/main.go)

![byte counters](examples/gifs/hIpTa3A5rQz65ssiVuRJu87X6.svg)
