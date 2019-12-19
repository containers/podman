# Multi Progress Bar

[![GoDoc](https://godoc.org/github.com/vbauerster/mpb?status.svg)](https://godoc.org/github.com/vbauerster/mpb)
[![Build Status](https://travis-ci.org/vbauerster/mpb.svg?branch=master)](https://travis-ci.org/vbauerster/mpb)
[![Go Report Card](https://goreportcard.com/badge/github.com/vbauerster/mpb)](https://goreportcard.com/report/github.com/vbauerster/mpb)

**mpb** is a Go lib for rendering progress bars in terminal applications.

## Features

* __Multiple Bars__: Multiple progress bars are supported
* __Dynamic Total__: Set total while bar is running
* __Dynamic Add/Remove__: Dynamically add or remove bars
* __Cancellation__: Cancel whole rendering process
* __Predefined Decorators__: Elapsed time, [ewma](https://github.com/VividCortex/ewma) based ETA, Percentage, Bytes counter
* __Decorator's width sync__:  Synchronized decorator's width among multiple bars

## Usage

#### [Rendering single bar](_examples/singleBar/main.go)
```go
package main

import (
    "math/rand"
    "time"

    "github.com/vbauerster/mpb/v4"
    "github.com/vbauerster/mpb/v4/decor"
)

func main() {
    // initialize progress container, with custom width
    p := mpb.New(mpb.WithWidth(64))

    total := 100
    name := "Single Bar:"
    // adding a single bar, which will inherit container's width
    bar := p.AddBar(int64(total),
        // override DefaultBarStyle, which is "[=>-]<+"
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
        // since ewma decorator is used, we need to pass time.Since(start)
        bar.Increment(time.Since(start))
    }
    // wait for our bar to complete and flush
    p.Wait()
}
```

#### [Rendering multiple bars](_examples/multiBars//main.go)
```go
    var wg sync.WaitGroup
    // pass &wg (optional), so p will wait for it eventually
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
            rng := rand.New(rand.NewSource(time.Now().UnixNano()))
            max := 100 * time.Millisecond
            for i := 0; i < total; i++ {
                start := time.Now()
                time.Sleep(time.Duration(rng.Intn(10)+1) * max / 10)
                // since ewma decorator is used, we need to pass time.Since(start)
                bar.Increment(time.Since(start))
            }
        }()
    }
    // Waiting for passed &wg and for all bars to complete and flush
    p.Wait()
```

#### [Dynamic total](_examples/dynTotal/main.go)

![dynamic total](_svg/godEMrCZmJkHYH1X9dN4Nm0U7.svg)

#### [Complex example](_examples/complex/main.go)

![complex](_svg/wHzf1M7sd7B3zVa2scBMnjqRf.svg)

#### [Bytes counters](_examples/io/main.go)

![byte counters](_svg/hIpTa3A5rQz65ssiVuRJu87X6.svg)
