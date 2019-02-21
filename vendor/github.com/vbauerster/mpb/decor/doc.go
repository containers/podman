// Copyright (C) 2016-2018 Vladimir Bauer
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
 Package decor contains common decorators used by "github.com/vbauerster/mpb" package.

 Some decorators returned by this package might have a closure state. It is ok to use
 decorators concurrently, unless you share the same decorator among multiple
 *mpb.Bar instances. To avoid data races, create new decorator per *mpb.Bar instance.

 Don't:

	 p := mpb.New()
	 name := decor.Name("bar")
	 p.AddBar(100, mpb.AppendDecorators(name))
	 p.AddBar(100, mpb.AppendDecorators(name))

 Do:

	p := mpb.New()
	p.AddBar(100, mpb.AppendDecorators(decor.Name("bar1")))
	p.AddBar(100, mpb.AppendDecorators(decor.Name("bar2")))
*/
package decor
