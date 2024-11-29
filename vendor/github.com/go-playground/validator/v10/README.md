Package validator
=================
<img align="right" src="logo.png">[![Join the chat at https://gitter.im/go-playground/validator](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/go-playground/validator?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
![Project status](https://img.shields.io/badge/version-10.20.0-green.svg)
[![Build Status](https://travis-ci.org/go-playground/validator.svg?branch=master)](https://travis-ci.org/go-playground/validator)
[![Coverage Status](https://coveralls.io/repos/go-playground/validator/badge.svg?branch=master&service=github)](https://coveralls.io/github/go-playground/validator?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-playground/validator)](https://goreportcard.com/report/github.com/go-playground/validator)
[![GoDoc](https://godoc.org/github.com/go-playground/validator?status.svg)](https://pkg.go.dev/github.com/go-playground/validator/v10)
![License](https://img.shields.io/dub/l/vibe-d.svg)

Package validator implements value validations for structs and individual fields based on tags.

It has the following **unique** features:

-   Cross Field and Cross Struct validations by using validation tags or custom validators.
-   Slice, Array and Map diving, which allows any or all levels of a multidimensional field to be validated.
-   Ability to dive into both map keys and values for validation
-   Handles type interface by determining it's underlying type prior to validation.
-   Handles custom field types such as sql driver Valuer see [Valuer](https://golang.org/src/database/sql/driver/types.go?s=1210:1293#L29)
-   Alias validation tags, which allows for mapping of several validations to a single tag for easier defining of validations on structs
-   Extraction of custom defined Field Name e.g. can specify to extract the JSON name while validating and have it available in the resulting FieldError
-   Customizable i18n aware error messages.
-   Default validator for the [gin](https://github.com/gin-gonic/gin) web framework; upgrading from v8 to v9 in gin see [here](https://github.com/go-playground/validator/tree/master/_examples/gin-upgrading-overriding)

Installation
------------

Use go get.

	go get github.com/go-playground/validator/v10

Then import the validator package into your own code.

	import "github.com/go-playground/validator/v10"

Error Return Value
-------

Validation functions return type error

They return type error to avoid the issue discussed in the following, where err is always != nil:

* http://stackoverflow.com/a/29138676/3158232
* https://github.com/go-playground/validator/issues/134

Validator returns only InvalidValidationError for bad validation input, nil or ValidationErrors as type error; so, in your code all you need to do is check if the error returned is not nil, and if it's not check if error is InvalidValidationError ( if necessary, most of the time it isn't ) type cast it to type ValidationErrors like so:

```go
err := validate.Struct(mystruct)
validationErrors := err.(validator.ValidationErrors)
 ```

Usage and documentation
------

Please see https://pkg.go.dev/github.com/go-playground/validator/v10 for detailed usage docs.

##### Examples:

- [Simple](https://github.com/go-playground/validator/blob/master/_examples/simple/main.go)
- [Custom Field Types](https://github.com/go-playground/validator/blob/master/_examples/custom/main.go)
- [Struct Level](https://github.com/go-playground/validator/blob/master/_examples/struct-level/main.go)
- [Translations & Custom Errors](https://github.com/go-playground/validator/blob/master/_examples/translations/main.go)
- [Gin upgrade and/or override validator](https://github.com/go-playground/validator/tree/v9/_examples/gin-upgrading-overriding)
- [wash - an example application putting it all together](https://github.com/bluesuncorp/wash)

Baked-in Validations
------

### Special Notes:
- If new to using validator it is highly recommended to initialize it using the `WithRequiredStructEnabled` option which is opt-in to new behaviour that will become the default behaviour in v11+. See documentation for more details.
```go
validate := validator.New(validator.WithRequiredStructEnabled())
```

### Fields:

| Tag | Description |
| - | - |
| eqcsfield | Field Equals Another Field (relative)|
| eqfield | Field Equals Another Field |
| fieldcontains | Check the indicated characters are present in the Field |
| fieldexcludes | Check the indicated characters are not present in the field |
| gtcsfield | Field Greater Than Another Relative Field |
| gtecsfield | Field Greater Than or Equal To Another Relative Field |
| gtefield | Field Greater Than or Equal To Another Field |
| gtfield | Field Greater Than Another Field |
| ltcsfield | Less Than Another Relative Field |
| ltecsfield | Less Than or Equal To Another Relative Field |
| ltefield | Less Than or Equal To Another Field |
| ltfield | Less Than Another Field |
| necsfield | Field Does Not Equal Another Field (relative) |
| nefield | Field Does Not Equal Another Field |

### Network:

| Tag | Description |
| - | - |
| cidr | Classless Inter-Domain Routing CIDR |
| cidrv4 | Classless Inter-Domain Routing CIDRv4 |
| cidrv6 | Classless Inter-Domain Routing CIDRv6 |
| datauri | Data URL |
| fqdn | Full Qualified Domain Name (FQDN) |
| hostname | Hostname RFC 952 |
| hostname_port | HostPort |
| hostname_rfc1123 | Hostname RFC 1123 |
| ip | Internet Protocol Address IP |
| ip4_addr | Internet Protocol Address IPv4 |
| ip6_addr | Internet Protocol Address IPv6 |
| ip_addr | Internet Protocol Address IP |
| ipv4 | Internet Protocol Address IPv4 |
| ipv6 | Internet Protocol Address IPv6 |
| mac | Media Access Control Address MAC |
| tcp4_addr | Transmission Control Protocol Address TCPv4 |
| tcp6_addr | Transmission Control Protocol Address TCPv6 |
| tcp_addr | Transmission Control Protocol Address TCP |
| udp4_addr | User Datagram Protocol Address UDPv4 |
| udp6_addr | User Datagram Protocol Address UDPv6 |
| udp_addr | User Datagram Protocol Address UDP |
| unix_addr | Unix domain socket end point Address |
| uri | URI String |
| url | URL String |
| http_url | HTTP URL String |
| url_encoded | URL Encoded |
| urn_rfc2141 | Urn RFC 2141 String |

### Strings:

| Tag | Description |
| - | - |
| alpha | Alpha Only |
| alphanum | Alphanumeric |
| alphanumunicode | Alphanumeric Unicode |
| alphaunicode | Alpha Unicode |
| ascii | ASCII |
| boolean | Boolean |
| contains | Contains |
| containsany | Contains Any |
| containsrune | Contains Rune |
| endsnotwith | Ends Not With |
| endswith | Ends With |
| excludes | Excludes |
| excludesall | Excludes All |
| excludesrune | Excludes Rune |
| lowercase | Lowercase |
| multibyte | Multi-Byte Characters |
| number | Number |
| numeric | Numeric |
| printascii | Printable ASCII |
| startsnotwith | Starts Not With |
| startswith | Starts With |
| uppercase | Uppercase |

### Format:
| Tag | Description |
| - | - |
| base64 | Base64 String |
| base64url | Base64URL String |
| base64rawurl | Base64RawURL String |
| bic | Business Identifier Code (ISO 9362) |
| bcp47_language_tag | Language tag (BCP 47) |
| btc_addr | Bitcoin Address |
| btc_addr_bech32 | Bitcoin Bech32 Address (segwit) |
| credit_card | Credit Card Number |
| mongodb | MongoDB ObjectID |
| cron | Cron |
| spicedb | SpiceDb ObjectID/Permission/Type |
| datetime | Datetime |
| e164 | e164 formatted phone number |
| email | E-mail String
| eth_addr | Ethereum Address |
| hexadecimal | Hexadecimal String |
| hexcolor | Hexcolor String |
| hsl | HSL String |
| hsla | HSLA String |
| html | HTML Tags |
| html_encoded | HTML Encoded |
| isbn | International Standard Book Number |
| isbn10 | International Standard Book Number 10 |
| isbn13 | International Standard Book Number 13 |
| issn | International Standard Serial Number |
| iso3166_1_alpha2 | Two-letter country code (ISO 3166-1 alpha-2) |
| iso3166_1_alpha3 | Three-letter country code (ISO 3166-1 alpha-3) |
| iso3166_1_alpha_numeric | Numeric country code (ISO 3166-1 numeric) |
| iso3166_2 | Country subdivision code (ISO 3166-2) |
| iso4217 | Currency code (ISO 4217) |
| json | JSON |
| jwt | JSON Web Token (JWT) |
| latitude | Latitude |
| longitude | Longitude |
| luhn_checksum | Luhn Algorithm Checksum (for strings and (u)int) |
| postcode_iso3166_alpha2 | Postcode |
| postcode_iso3166_alpha2_field | Postcode |
| rgb | RGB String |
| rgba | RGBA String |
| ssn | Social Security Number SSN |
| timezone | Timezone |
| uuid | Universally Unique Identifier UUID |
| uuid3 | Universally Unique Identifier UUID v3 |
| uuid3_rfc4122 | Universally Unique Identifier UUID v3 RFC4122 |
| uuid4 | Universally Unique Identifier UUID v4 |
| uuid4_rfc4122 | Universally Unique Identifier UUID v4 RFC4122 |
| uuid5 | Universally Unique Identifier UUID v5 |
| uuid5_rfc4122 | Universally Unique Identifier UUID v5 RFC4122 |
| uuid_rfc4122 | Universally Unique Identifier UUID RFC4122 |
| md4 | MD4 hash |
| md5 | MD5 hash |
| sha256 | SHA256 hash |
| sha384 | SHA384 hash |
| sha512 | SHA512 hash |
| ripemd128 | RIPEMD-128 hash |
| ripemd128 | RIPEMD-160 hash |
| tiger128 | TIGER128 hash |
| tiger160 | TIGER160 hash |
| tiger192 | TIGER192 hash |
| semver | Semantic Versioning 2.0.0 |
| ulid | Universally Unique Lexicographically Sortable Identifier ULID |
| cve | Common Vulnerabilities and Exposures Identifier (CVE id) |

### Comparisons:
| Tag | Description |
| - | - |
| eq | Equals |
| eq_ignore_case | Equals ignoring case |
| gt | Greater than|
| gte | Greater than or equal |
| lt | Less Than |
| lte | Less Than or Equal |
| ne | Not Equal |
| ne_ignore_case | Not Equal ignoring case |

### Other:
| Tag | Description |
| - | - |
| dir | Existing Directory |
| dirpath | Directory Path |
| file | Existing File |
| filepath | File Path |
| image | Image |
| isdefault | Is Default |
| len | Length |
| max | Maximum |
| min | Minimum |
| oneof | One Of |
| required | Required |
| required_if | Required If |
| required_unless | Required Unless |
| required_with | Required With |
| required_with_all | Required With All |
| required_without | Required Without |
| required_without_all | Required Without All |
| excluded_if | Excluded If |
| excluded_unless | Excluded Unless |
| excluded_with | Excluded With |
| excluded_with_all | Excluded With All |
| excluded_without | Excluded Without |
| excluded_without_all | Excluded Without All |
| unique | Unique |

#### Aliases:
| Tag | Description |
| - | - |
| iscolor | hexcolor\|rgb\|rgba\|hsl\|hsla |
| country_code | iso3166_1_alpha2\|iso3166_1_alpha3\|iso3166_1_alpha_numeric |

Benchmarks
------
###### Run on MacBook Pro (15-inch, 2017) go version go1.10.2 darwin/amd64
```go
go version go1.21.0 darwin/arm64
goos: darwin
goarch: arm64
pkg: github.com/go-playground/validator/v10
BenchmarkFieldSuccess-8                                         33142266                35.94 ns/op            0 B/op          0 allocs/op
BenchmarkFieldSuccessParallel-8                                 200816191                6.568 ns/op           0 B/op          0 allocs/op
BenchmarkFieldFailure-8                                          6779707               175.1 ns/op           200 B/op          4 allocs/op
BenchmarkFieldFailureParallel-8                                 11044147               108.4 ns/op           200 B/op          4 allocs/op
BenchmarkFieldArrayDiveSuccess-8                                 6054232               194.4 ns/op            97 B/op          5 allocs/op
BenchmarkFieldArrayDiveSuccessParallel-8                        12523388                94.07 ns/op           97 B/op          5 allocs/op
BenchmarkFieldArrayDiveFailure-8                                 3587043               334.3 ns/op           300 B/op         10 allocs/op
BenchmarkFieldArrayDiveFailureParallel-8                         5816665               200.8 ns/op           300 B/op         10 allocs/op
BenchmarkFieldMapDiveSuccess-8                                   2217910               540.1 ns/op           288 B/op         14 allocs/op
BenchmarkFieldMapDiveSuccessParallel-8                           4446698               258.7 ns/op           288 B/op         14 allocs/op
BenchmarkFieldMapDiveFailure-8                                   2392759               504.6 ns/op           376 B/op         13 allocs/op
BenchmarkFieldMapDiveFailureParallel-8                           4244199               286.9 ns/op           376 B/op         13 allocs/op
BenchmarkFieldMapDiveWithKeysSuccess-8                           2005857               592.1 ns/op           288 B/op         14 allocs/op
BenchmarkFieldMapDiveWithKeysSuccessParallel-8                   4400850               296.9 ns/op           288 B/op         14 allocs/op
BenchmarkFieldMapDiveWithKeysFailure-8                           1850227               643.8 ns/op           553 B/op         16 allocs/op
BenchmarkFieldMapDiveWithKeysFailureParallel-8                   3293233               375.1 ns/op           553 B/op         16 allocs/op
BenchmarkFieldCustomTypeSuccess-8                               12174412                98.25 ns/op           32 B/op          2 allocs/op
BenchmarkFieldCustomTypeSuccessParallel-8                       34389907                35.49 ns/op           32 B/op          2 allocs/op
BenchmarkFieldCustomTypeFailure-8                                7582524               156.6 ns/op           184 B/op          3 allocs/op
BenchmarkFieldCustomTypeFailureParallel-8                       13019902                92.79 ns/op          184 B/op          3 allocs/op
BenchmarkFieldOrTagSuccess-8                                     3427260               349.4 ns/op            16 B/op          1 allocs/op
BenchmarkFieldOrTagSuccessParallel-8                            15144128                81.25 ns/op           16 B/op          1 allocs/op
BenchmarkFieldOrTagFailure-8                                     5913546               201.9 ns/op           216 B/op          5 allocs/op
BenchmarkFieldOrTagFailureParallel-8                             9810212               113.7 ns/op           216 B/op          5 allocs/op
BenchmarkStructLevelValidationSuccess-8                         13456327                87.66 ns/op           16 B/op          1 allocs/op
BenchmarkStructLevelValidationSuccessParallel-8                 41818888                27.77 ns/op           16 B/op          1 allocs/op
BenchmarkStructLevelValidationFailure-8                          4166284               272.6 ns/op           264 B/op          7 allocs/op
BenchmarkStructLevelValidationFailureParallel-8                  7594581               152.1 ns/op           264 B/op          7 allocs/op
BenchmarkStructSimpleCustomTypeSuccess-8                         6508082               182.6 ns/op            32 B/op          2 allocs/op
BenchmarkStructSimpleCustomTypeSuccessParallel-8                23078605                54.78 ns/op           32 B/op          2 allocs/op
BenchmarkStructSimpleCustomTypeFailure-8                         3118352               381.0 ns/op           416 B/op          9 allocs/op
BenchmarkStructSimpleCustomTypeFailureParallel-8                 5300738               224.1 ns/op           432 B/op         10 allocs/op
BenchmarkStructFilteredSuccess-8                                 4761807               251.1 ns/op           216 B/op          5 allocs/op
BenchmarkStructFilteredSuccessParallel-8                         8792598               128.6 ns/op           216 B/op          5 allocs/op
BenchmarkStructFilteredFailure-8                                 5202573               232.1 ns/op           216 B/op          5 allocs/op
BenchmarkStructFilteredFailureParallel-8                         9591267               121.4 ns/op           216 B/op          5 allocs/op
BenchmarkStructPartialSuccess-8                                  5188512               231.6 ns/op           224 B/op          4 allocs/op
BenchmarkStructPartialSuccessParallel-8                          9179776               123.1 ns/op           224 B/op          4 allocs/op
BenchmarkStructPartialFailure-8                                  3071212               392.5 ns/op           440 B/op          9 allocs/op
BenchmarkStructPartialFailureParallel-8                          5344261               223.7 ns/op           440 B/op          9 allocs/op
BenchmarkStructExceptSuccess-8                                   3184230               375.0 ns/op           424 B/op          8 allocs/op
BenchmarkStructExceptSuccessParallel-8                          10090130               108.9 ns/op           208 B/op          3 allocs/op
BenchmarkStructExceptFailure-8                                   3347226               357.7 ns/op           424 B/op          8 allocs/op
BenchmarkStructExceptFailureParallel-8                           5654923               209.5 ns/op           424 B/op          8 allocs/op
BenchmarkStructSimpleCrossFieldSuccess-8                         5232265               229.1 ns/op            56 B/op          3 allocs/op
BenchmarkStructSimpleCrossFieldSuccessParallel-8                17436674                64.75 ns/op           56 B/op          3 allocs/op
BenchmarkStructSimpleCrossFieldFailure-8                         3128613               383.6 ns/op           272 B/op          8 allocs/op
BenchmarkStructSimpleCrossFieldFailureParallel-8                 6994113               168.8 ns/op           272 B/op          8 allocs/op
BenchmarkStructSimpleCrossStructCrossFieldSuccess-8              3506487               340.9 ns/op            64 B/op          4 allocs/op
BenchmarkStructSimpleCrossStructCrossFieldSuccessParallel-8     13431300                91.77 ns/op           64 B/op          4 allocs/op
BenchmarkStructSimpleCrossStructCrossFieldFailure-8              2410566               500.9 ns/op           288 B/op          9 allocs/op
BenchmarkStructSimpleCrossStructCrossFieldFailureParallel-8      6344510               188.2 ns/op           288 B/op          9 allocs/op
BenchmarkStructSimpleSuccess-8                                   8922726               133.8 ns/op             0 B/op          0 allocs/op
BenchmarkStructSimpleSuccessParallel-8                          55291153                23.63 ns/op            0 B/op          0 allocs/op
BenchmarkStructSimpleFailure-8                                   3171553               378.4 ns/op           416 B/op          9 allocs/op
BenchmarkStructSimpleFailureParallel-8                           5571692               212.0 ns/op           416 B/op          9 allocs/op
BenchmarkStructComplexSuccess-8                                  1683750               714.5 ns/op           224 B/op          5 allocs/op
BenchmarkStructComplexSuccessParallel-8                          4578046               257.0 ns/op           224 B/op          5 allocs/op
BenchmarkStructComplexFailure-8                                   481585              2547 ns/op            3041 B/op         48 allocs/op
BenchmarkStructComplexFailureParallel-8                           965764              1577 ns/op            3040 B/op         48 allocs/op
BenchmarkOneof-8                                                17380881                68.50 ns/op            0 B/op          0 allocs/op
BenchmarkOneofParallel-8                                         8084733               153.5 ns/op             0 B/op          0 allocs/op
```

Complementary Software
----------------------

Here is a list of software that complements using this library either pre or post validation.

* [form](https://github.com/go-playground/form) - Decodes url.Values into Go value(s) and Encodes Go value(s) into url.Values. Dual Array and Full map support.
* [mold](https://github.com/go-playground/mold) - A general library to help modify or set data within data structures and other objects

How to Contribute
------

Make a pull request...

License
-------
Distributed under MIT License, please see license file within the code for more details.

Maintainers
-----------
This project has grown large enough that more than one person is required to properly support the community.
If you are interested in becoming a maintainer please reach out to me https://github.com/deankarn
