# Deepcopier

[![Build Status](https://secure.travis-ci.org/ulule/deepcopier.png?branch=master)](http://travis-ci.org/ulule/deepcopier)

This package is meant to make copying of structs to/from others structs a bit easier.

## Installation

```bash
go get -u github.com/ulule/deepcopier
```

## Usage

```golang
// Deep copy instance1 into instance2
Copy(instance1).To(instance2)

// Deep copy instance1 into instance2 and passes the following context (which
// is basically a map[string]interface{}) as first argument
// to methods of instance2 that defined the struct tag "context".
Copy(instance1).WithContext(map[string]interface{}{"foo": "bar"}).To(instance2)

// Deep copy instance2 into instance1
Copy(instance1).From(instance2)

// Deep copy instance2 into instance1 and passes the following context (which
// is basically a map[string]interface{}) as first argument
// to methods of instance1 that defined the struct tag "context".
Copy(instance1).WithContext(map[string]interface{}{"foo": "bar"}).From(instance2)
```

Available options for `deepcopier` struct tag:

| Option    | Description                                                          |
| --------- | -------------------------------------------------------------------- |
| `field`   | Field or method name in source instance                              |
| `skip`    | Ignores the field                                                    |
| `context` | Takes a `map[string]interface{}` as first argument (for methods)     |
| `force`   | Set the value of a `sql.Null*` field (instead of copying the struct) |

**Options example:**

```golang
type Source struct {
    Name                         string
    SkipMe                       string
    SQLNullStringToSQLNullString sql.NullString
    SQLNullStringToString        sql.NullString

}

func (Source) MethodThatTakesContext(c map[string]interface{}) string {
    return "whatever"
}

type Destination struct {
    FieldWithAnotherNameInSource      string         `deepcopier:"field:Name"`
    SkipMe                            string         `deepcopier:"skip"`
    MethodThatTakesContext            string         `deepcopier:"context"`
    SQLNullStringToSQLNullString      sql.NullString 
    SQLNullStringToString             string         `deepcopier:"force"`
}

```

Example:

```golang
package main

import (
    "fmt"
 
    "github.com/ulule/deepcopier"
)

// Model
type User struct {
    // Basic string field
    Name  string
    // Deepcopier supports https://golang.org/pkg/database/sql/driver/#Valuer
    Email sql.NullString
}

func (u *User) MethodThatTakesContext(ctx map[string]interface{}) string {
    // do whatever you want
    return "hello from this method"
}

// Resource
type UserResource struct {
    DisplayName            string `deepcopier:"field:Name"`
    SkipMe                 string `deepcopier:"skip"`
    MethodThatTakesContext string `deepcopier:"context"`
    Email                  string `deepcopier:"force"`

}

func main() {
    user := &User{
        Name: "gilles",
        Email: sql.NullString{
            Valid: true,
            String: "gilles@example.com",
        },
    }

    resource := &UserResource{}

    deepcopier.Copy(user).To(resource)

    fmt.Println(resource.DisplayName)
    fmt.Println(resource.Email)
}
```

Looking for more information about the usage?

We wrote [an introduction article](https://github.com/ulule/deepcopier/blob/master/examples/rest-usage/README.rst).
Have a look and feel free to give us your feedback.

## Contributing

* Ping us on twitter [@oibafsellig](https://twitter.com/oibafsellig), [@thoas](https://twitter.com/thoas)
* Fork the [project](https://github.com/ulule/deepcopier)
* Help us improving and fixing [issues](https://github.com/ulule/deepcopier/issues)

Don't hesitate ;)
