# compress

This package is based on an optimized Deflate function, which is used by gzip/zip/zlib packages.

It offers slightly better compression at lower compression settings, and up to 3x faster encoding at highest compression level.

* [High Throughput Benchmark](http://blog.klauspost.com/go-gzipdeflate-benchmarks/).
* [Small Payload/Webserver Benchmarks](http://blog.klauspost.com/gzip-performance-for-go-webservers/).
* [Linear Time Compression](http://blog.klauspost.com/constant-time-gzipzip-compression/).
* [Re-balancing Deflate Compression Levels](https://blog.klauspost.com/rebalancing-deflate-compression-levels/)

[![Build Status](https://travis-ci.org/klauspost/compress.svg?branch=master)](https://travis-ci.org/klauspost/compress)
[![Sourcegraph Badge](https://sourcegraph.com/github.com/klauspost/compress/-/badge.svg)](https://sourcegraph.com/github.com/klauspost/compress?badge)

# changelog

* Aug 1, 2018: Added [huff0 README](https://github.com/klauspost/compress/tree/master/huff0#huff0-entropy-compression).
* Jul 8, 2018: Added [Performance Update 2018](#performance-update-2018) below.
* Jun 23, 2018: Merged [Go 1.11 inflate optimizations](https://go-review.googlesource.com/c/go/+/102235). Go 1.9 is now required. Backwards compatible version tagged with [v1.3.0](https://github.com/klauspost/compress/releases/tag/v1.3.0).
* Apr 2, 2018: Added [huff0](https://godoc.org/github.com/klauspost/compress/huff0) en/decoder. Experimental for now, API may change.
* Mar 4, 2018: Added [FSE Entropy](https://godoc.org/github.com/klauspost/compress/fse) en/decoder. Experimental for now, API may change.
* Nov 3, 2017: Add compression [Estimate](https://godoc.org/github.com/klauspost/compress#Estimate) function.
* May 28, 2017: Reduce allocations when resetting decoder.
* Apr 02, 2017: Change back to official crc32, since changes were merged in Go 1.7.
* Jan 14, 2017: Reduce stack pressure due to array copies. See [Issue #18625](https://github.com/golang/go/issues/18625).
* Oct 25, 2016: Level 2-4 have been rewritten and now offers significantly better performance than before.
* Oct 20, 2016: Port zlib changes from Go 1.7 to fix zlib writer issue. Please update.
* Oct 16, 2016: Go 1.7 changes merged. Apples to apples this package is a few percent faster, but has a significantly better balance between speed and compression per level. 
* Mar 24, 2016: Always attempt Huffman encoding on level 4-7. This improves base 64 encoded data compression.
* Mar 24, 2016: Small speedup for level 1-3.
* Feb 19, 2016: Faster bit writer, level -2 is 15% faster, level 1 is 4% faster.
* Feb 19, 2016: Handle small payloads faster in level 1-3.
* Feb 19, 2016: Added faster level 2 + 3 compression modes.
* Feb 19, 2016: [Rebalanced compression levels](https://blog.klauspost.com/rebalancing-deflate-compression-levels/), so there is a more even progresssion in terms of compression. New default level is 5.
* Feb 14, 2016: Snappy: Merge upstream changes. 
* Feb 14, 2016: Snappy: Fix aggressive skipping.
* Feb 14, 2016: Snappy: Update benchmark.
* Feb 13, 2016: Deflate: Fixed assembler problem that could lead to sub-optimal compression.
* Feb 12, 2016: Snappy: Added AMD64 SSE 4.2 optimizations to matching, which makes easy to compress material run faster. Typical speedup is around 25%.
* Feb 9, 2016: Added Snappy package fork. This version is 5-7% faster, much more on hard to compress content.
* Jan 30, 2016: Optimize level 1 to 3 by not considering static dictionary or storing uncompressed. ~4-5% speedup.
* Jan 16, 2016: Optimization on deflate level 1,2,3 compression.
* Jan 8 2016: Merge [CL 18317](https://go-review.googlesource.com/#/c/18317): fix reading, writing of zip64 archives.
* Dec 8 2015: Make level 1 and -2 deterministic even if write size differs.
* Dec 8 2015: Split encoding functions, so hashing and matching can potentially be inlined. 1-3% faster on AMD64. 5% faster on other platforms.
* Dec 8 2015: Fixed rare [one byte out-of bounds read](https://github.com/klauspost/compress/issues/20). Please update!
* Nov 23 2015: Optimization on token writer. ~2-4% faster. Contributed by [@dsnet](https://github.com/dsnet).
* Nov 20 2015: Small optimization to bit writer on 64 bit systems.
* Nov 17 2015: Fixed out-of-bound errors if the underlying Writer returned an error. See [#15](https://github.com/klauspost/compress/issues/15).
* Nov 12 2015: Added [io.WriterTo](https://golang.org/pkg/io/#WriterTo) support to gzip/inflate.
* Nov 11 2015: Merged [CL 16669](https://go-review.googlesource.com/#/c/16669/4): archive/zip: enable overriding (de)compressors per file
* Oct 15 2015: Added skipping on uncompressible data. Random data speed up >5x.

# usage

The packages are drop-in replacements for standard libraries. Simply replace the import path to use them:

| old import         | new import                              |
|--------------------|-----------------------------------------|
| `compress/gzip`    | `github.com/klauspost/compress/gzip`    |
| `compress/zlib`    | `github.com/klauspost/compress/zlib`    |
| `archive/zip`      | `github.com/klauspost/compress/zip`     |
| `compress/flate`   | `github.com/klauspost/compress/flate`   |

You may also be interested in [pgzip](https://github.com/klauspost/pgzip), which is a drop in replacement for gzip, which support multithreaded compression on big files and the optimized [crc32](https://github.com/klauspost/crc32) package used by these packages.

The packages contains the same as the standard library, so you can use the godoc for that: [gzip](http://golang.org/pkg/compress/gzip/), [zip](http://golang.org/pkg/archive/zip/),  [zlib](http://golang.org/pkg/compress/zlib/), [flate](http://golang.org/pkg/compress/flate/).

Currently there is only minor speedup on decompression (mostly CRC32 calculation).

# Performance Update 2018

It has been a while since we have been looking at the speed of this package compared to the standard library, so I thought I would re-do my tests and give some overall recommendations based on the current state. All benchmarks have been performed with Go 1.10 on my Desktop Intel(R) Core(TM) i7-2600 CPU @3.40GHz. Since I last ran the tests, I have gotten more RAM, which means tests with big files are no longer limited by my SSD.

The raw results are in my [updated spreadsheet](https://docs.google.com/spreadsheets/d/1nuNE2nPfuINCZJRMt6wFWhKpToF95I47XjSsc-1rbPQ/edit?usp=sharing). Due to cgo changes and upstream updates i could not get the cgo version of gzip to compile. Instead I included the [zstd](https://github.com/datadog/zstd) cgo implementation. If I get cgo gzip to work again, I might replace the results in the sheet.

The columns to take note of are: *MB/s* - the throughput. *Reduction* - the data size reduction in percent of the original. *Rel Speed* relative speed compared to the standard libary at the same level. *Smaller* - how many percent smaller is the compressed output compared to stdlib. Negative means the output was bigger. *Loss* means the loss (or gain) in compression as a percentage difference of the input.

The `gzstd` (standard library gzip) and `gzkp` (this package gzip) only uses one CPU core. [`pgzip`](https://github.com/klauspost/pgzip), [`bgzf`](https://github.com/biogo/hts/bgzf) uses all 4 cores. [`zstd`](https://github.com/DataDog/zstd) uses one core, and is a beast (but not Go, yet).


## Overall differences.

There appears to be a roughly 5-10% speed advantage over the standard library when comparing at similar compression levels.

The biggest difference you will see is the result of [re-balancing](https://blog.klauspost.com/rebalancing-deflate-compression-levels/) the compression levels. I wanted by library to give a smoother transition between the compression levels than the standard library.

This package attempts to provide a more smooth transition, where "1" is taking a lot of shortcuts, "5" is the reasonable trade-off and "9" is the "give me the best compression", and the values in between gives something reasonable in between. The standard library has big differences in levels 1-4, but levels 5-9 having no significant gains - often spending a lot more time than can be justified by the achieved compression.

There are links to all the test data in the [spreadsheet](https://docs.google.com/spreadsheets/d/1nuNE2nPfuINCZJRMt6wFWhKpToF95I47XjSsc-1rbPQ/edit?usp=sharing) in the top left field on each tab.

## Web Content

This test set aims to emulate typical use in a web server. The test-set is 4GB data in 53k files, and is a mixture of (mostly) HTML, JS, CSS.

Since level 1 and 9 are close to being the same code, they are quite close. But looking at the levels in-between the differences are quite big.

Looking at level 6, this package is 88% faster, but will output about 6% more data. For a web server, this means you can serve 88% more data, but have to pay for 6% more bandwidth. You can draw your own conclusions on what would be the most expensive for your case.

## Object files

This test is for typical data files stored on a server. In this case it is a collection of Go precompiled objects. They are very compressible.

The picture is similar to the web content, but with small differences since this is very compressible. Levels 2-3 offer good speed, but is sacrificing quite a bit of compression. 

The standard library seems suboptimal on level 3 and 4 - offering both worse compression and speed than level 6 & 7 of this package respectively.

## Highly Compressible File

This is a JSON file with very high redundancy. The reduction starts at 95% on level 1, so in real life terms we are dealing with something like a highly redundant stream of data, etc.

It is definitely visible that we are dealing with specialized content here, so the results are very scattered. This package does not do very well at levels 1-4, but picks up significantly at level 5 and levels 7 and 8 offering great speed for the achieved compression.

So if you know you content is extremely compressible you might want to go slightly higher than the defaults. The standard library has a huge gap between levels 3 and 4 in terms of speed (2.75x slowdown), so it offers little "middle ground".

## Medium-High Compressible

This is a pretty common test corpus: [enwik9](http://mattmahoney.net/dc/textdata.html). It contains the first 10^9 bytes of the English Wikipedia dump on Mar. 3, 2006. This is a very good test of typical text based compression and more data heavy streams.

We see a similar picture here as in "Web Content". On equal levels some compression is sacrificed for more speed. Level 5 seems to be the best trade-off between speed and size, beating stdlib level 3 in both.

## Medium Compressible

I will combine two test sets, one [10GB file set](http://mattmahoney.net/dc/10gb.html) and a VM disk image (~8GB). Both contain different data types and represent a typical backup scenario.

The most notable thing is how quickly the standard libary drops to very low compression speeds around level 5-6 without any big gains in compression. Since this type of data is fairly common, this does not seem like good behavior.


## Un-compressible Content

This is mainly a test of how good the algorithms are at detecting un-compressible input. The standard library only offers this feature with very conservative settings at level 1. Obviously there is no reason for the algorithms to try to compress input that cannot be compressed.  The only downside is that it might skip some compressible data on false detections.


# linear time compression (huffman only)

This compression library adds a special compression level, named `HuffmanOnly`, which allows near linear time compression. This is done by completely disabling matching of previous data, and only reduce the number of bits to represent each character. 

This means that often used characters, like 'e' and ' ' (space) in text use the fewest bits to represent, and rare characters like '¤' takes more bits to represent. For more information see [wikipedia](https://en.wikipedia.org/wiki/Huffman_coding) or this nice [video](https://youtu.be/ZdooBTdW5bM).

Since this type of compression has much less variance, the compression speed is mostly unaffected by the input data, and is usually more than *180MB/s* for a single core.

The downside is that the compression ratio is usually considerably worse than even the fastest conventional compression. The compression raio can never be better than 8:1 (12.5%). 

The linear time compression can be used as a "better than nothing" mode, where you cannot risk the encoder to slow down on some content. For comparison, the size of the "Twain" text is *233460 bytes* (+29% vs. level 1) and encode speed is 144MB/s (4.5x level 1). So in this case you trade a 30% size increase for a 4 times speedup.

For more information see my blog post on [Fast Linear Time Compression](http://blog.klauspost.com/constant-time-gzipzip-compression/).

This is implemented on Go 1.7 as "Huffman Only" mode, though not exposed for gzip.


# snappy package

The standard snappy package has now been improved. This repo contains a copy of the snappy repo.

I would advise to use the standard package: https://github.com/golang/snappy


# license

This code is licensed under the same conditions as the original Go code. See LICENSE file.
