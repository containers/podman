# Throttler - intelligent WaitGroups

[![GoDoc](https://godoc.org/github.com/nozzle/throttler?status.svg)](http://godoc.org/github.com/nozzle/throttler) [![Coverage Status](https://coveralls.io/repos/nozzle/throttler/badge.svg?branch=master)](https://coveralls.io/r/nozzle/throttler?branch=master)


 Throttler fills the gap between sync.WaitGroup and manually monitoring your goroutines with channels. The API is almost identical to Wait Groups, but it allows you to set a max number of workers that can be running simultaneously. It uses channels internally to block until a job completes by calling Done() or until all jobs have been completed. It also provides a built in error channel that captures your goroutine errors and provides access to them as `[]error` after you exit the loop.

See a fully functional example on the playground at http://bit.ly/throttler-v3

Compare the Throttler example to the sync.WaitGroup example from http://golang.org/pkg/sync/#example_WaitGroup

### How to use Throttler

```
// This example fetches several URLs concurrently,
// using a Throttler to block until all the fetches are complete.
// Compare to http://golang.org/pkg/sync/#example_WaitGroup
func ExampleThrottler() {
	var urls = []string{
		"http://www.golang.org/",
		"http://www.google.com/",
		"http://www.somestupidname.com/",
	}
	// Create a new Throttler that will get 2 urls at a time
	t := throttler.New(2, len(urls))
	for _, url := range urls {
		// Launch a goroutine to fetch the URL.
		go func(url string) {
			// Fetch the URL.
			err := http.Get(url)
			// Let Throttler know when the goroutine completes
			// so it can dispatch another worker
			t.Done(err)
		}(url)
		// Pauses until a worker is available or all jobs have been completed
		// Returning the total number of goroutines that have errored
		// lets you choose to break out of the loop without starting any more
		errorCount := t.Throttle()
	}
}
```

### vs How to use a sync.WaitGroup

```
// This example fetches several URLs concurrently,
// using a WaitGroup to block until all the fetches are complete.
func ExampleWaitGroup() {
	var wg sync.WaitGroup
	var urls = []string{
		"http://www.golang.org/",
		"http://www.google.com/",
		"http://www.somestupidname.com/",
	}
	for _, url := range urls {
		// Increment the WaitGroup counter.
		wg.Add(1)
		// Launch a goroutine to fetch the URL.
		go func(url string) {
			// Decrement the counter when the goroutine completes.
			defer wg.Done()
			// Fetch the URL.
			http.Get(url)
		}(url)
	}
	// Wait for all HTTP fetches to complete.
	wg.Wait()
}
```