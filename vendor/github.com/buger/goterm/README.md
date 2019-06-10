## Description

This library provides basic building blocks for building advanced console UIs.

Initially created for [Gor](http://github.com/buger/gor).

Full API documentation: http://godoc.org/github.com/buger/goterm

## Basic usage

Full screen console app, printing current time:

```go
import (
    tm "github.com/buger/goterm"
    "time"
)

func main() {
    tm.Clear() // Clear current screen

    for {
        // By moving cursor to top-left position we ensure that console output
        // will be overwritten each time, instead of adding new.
        tm.MoveCursor(1,1)

        tm.Println("Current Time:", time.Now().Format(time.RFC1123))

        tm.Flush() // Call it every time at the end of rendering

        time.Sleep(time.Second)
    }
}
```

This can be seen in [examples/time_example.go](examples/time_example.go).  To
run it yourself, go into your `$GOPATH/src/github.com/buger/goterm` directory
and run `go run ./examples/time_example.go`


Print red bold message on white background:

```go    
tm.Println(tm.Background(tm.Color(tm.Bold("Important header"), tm.RED), tm.WHITE))
```


Create box and move it to center of the screen:

```go
tm.Clear()

// Create Box with 30% width of current screen, and height of 20 lines
box := tm.NewBox(30|tm.PCT, 20, 0)

// Add some content to the box
// Note that you can add ANY content, even tables
fmt.Fprint(box, "Some box content")

// Move Box to approx center of the screen
tm.Print(tm.MoveTo(box.String(), 40|tm.PCT, 40|tm.PCT))

tm.Flush()
```

This can be found in [examples/box_example.go](examples/box_example.go).

Draw table:

```go
// Based on http://golang.org/pkg/text/tabwriter
totals := tm.NewTable(0, 10, 5, ' ', 0)
fmt.Fprintf(totals, "Time\tStarted\tActive\tFinished\n")
fmt.Fprintf(totals, "%s\t%d\t%d\t%d\n", "All", started, started-finished, finished)
tm.Println(totals)
tm.Flush()
```

This can be found in [examples/table_example.go](examples/table_example.go).

## Line charts

Chart example:

![screen shot 2013-07-09 at 5 05 37 pm](https://f.cloud.github.com/assets/14009/767676/e3dd35aa-e887-11e2-9cd2-f6451eb26adc.png)


```go
    import (
        tm "github.com/buger/goterm"
    )

    chart := tm.NewLineChart(100, 20)
    
    data := new(tm.DataTable)
    data.AddColumn("Time")
    data.AddColumn("Sin(x)")
    data.AddColumn("Cos(x+1)")

    for i := 0.1; i < 10; i += 0.1 {
	data.AddRow(i, math.Sin(i), math.Cos(i+1))
    }
    
    tm.Println(chart.Draw(data))
```

This can be found in [examples/chart_example.go](examples/chart_example.go).

Drawing 2 separate graphs in different scales. Each graph have its own Y axe.

```go
chart.Flags = tm.DRAW_INDEPENDENT
```

Drawing graph with relative scale (Grapwh draw starting from min value instead of zero)

```go
chart.Flags = tm.DRAW_RELATIVE
```
