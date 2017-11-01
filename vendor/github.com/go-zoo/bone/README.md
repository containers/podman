bone [![GoDoc](https://godoc.org/github.com/squiidz/bone?status.png)](http://godoc.org/github.com/go-zoo/bone) [![Build Status](https://travis-ci.org/go-zoo/bone.svg)](https://travis-ci.org/go-zoo/bone) [![Go Report Card](https://goreportcard.com/badge/go-zoo/bone)](https://goreportcard.com/report/go-zoo/bone) [![Sourcegraph](https://sourcegraph.com/github.com/go-zoo/bone/-/badge.svg)](https://sourcegraph.com/github.com/go-zoo/bone?badge)
=======

## What is bone ?

Bone is a lightweight and lightning fast HTTP Multiplexer for Golang. It support :

- URL Parameters
- REGEX Parameters
- Wildcard routes
- Router Prefix
- Sub Router, `mux.SubRoute()`, support most standard router (bone, gorilla/mux, httpRouter etc...)
- Http method declaration
- Support for `http.Handler` and `http.HandlerFunc`
- Custom NotFound handler
- Respect the Go standard `http.Handler` interface

![alt tag](https://c2.staticflickr.com/2/1070/540747396_5542b42cca_z.jpg)

## Speed

```
- BenchmarkBoneMux        10000000               118 ns/op
- BenchmarkZeusMux          100000               144 ns/op
- BenchmarkHttpRouterMux  10000000               134 ns/op
- BenchmarkNetHttpMux      3000000               580 ns/op
- BenchmarkGorillaMux       300000              3333 ns/op
- BenchmarkGorillaPatMux   1000000              1889 ns/op
```

 These test are just for fun, all these router are great and really efficient.
 Bone do not pretend to be the fastest router for every job.

## Example

``` go

package main

import(
  "net/http"

  "github.com/go-zoo/bone"
)

func main () {
  mux := bone.New()

  // mux.Get, Post, etc ... takes http.Handler
  mux.Get("/home/:id", http.HandlerFunc(HomeHandler))
  mux.Get("/profil/:id/:var", http.HandlerFunc(ProfilHandler))
  mux.Post("/data", http.HandlerFunc(DataHandler))

  // Support REGEX Route params
  mux.Get("/index/#id^[0-9]$", http.HandlerFunc(IndexHandler))

  // Handle take http.Handler
  mux.Handle("/", http.HandlerFunc(RootHandler))

  // GetFunc, PostFunc etc ... takes http.HandlerFunc
  mux.GetFunc("/test", Handler)

  http.ListenAndServe(":8080", mux)
}

func Handler(rw http.ResponseWriter, req *http.Request) {
  // Get the value of the "id" parameters.
  val := bone.GetValue(req, "id")

  rw.Write([]byte(val))
}

```

## Blog Posts
- http://www.peterbe.com/plog/my-favorite-go-multiplexer
- https://harshladha.xyz/my-first-library-in-go-language-hasty-791b8e2b9e69

## Libs
- Errors dump for Go : [Trash](https://github.com/go-zoo/trash)
- Middleware Chaining module : [Claw](https://github.com/go-zoo/claw)
