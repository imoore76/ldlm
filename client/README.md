# Go LDLM Client

<p align="center">
<img src="../images/LDLM%20Logo%20Symbol.png" alt="ldlm logo" />
</p>


A Go <a href="http://github.com/imoore76/ldlm" target="_blank">LDLM</a> client library.
For LDLM concepts, use cases, and general information, see the
<a href="https://ldlm.readthedocs.io/" target="_blank">LDLM documentation</a>.

Installation
===============

```go
// your go application
include "github.com/imoore76/ldlm/client"
```

then

```bash
$ go mod tidy
```

## Basic Example

```go
import (
    "context"

    "github.com/imoore76/ldlm/client"
)

c, _ := client.New(context.Background(), client.Config{
    Address: "ldlm-server:3144",
})

lock, err := c.Lock("my-task", nil)

if err != nil {
    panic(err)
}

doMyTask()

if err = lock.Unlock(); err != nil {
    panic(err)
}
```


## Usage

Comprehensive API documentation is available at
<a href="https://pkg.go.dev/github.com/imoore76/ldlm/client" target="_blank">pkg.go.dev</a>.

### Basic Concepts

Be sure to read about LDLM lock concepts in the 
<a href="https://ldlm.readthedocs.io/en/stable/concepts.html" target="_blank">`LDLM documentation</a>.

### Create a Client
A client takes a context and a client.Config object. You can Cancel the context to abort a client's operations.

```go
c, _ := client.New(context.Background(), client.Config{
    Address: "localhost:3144",
})
```

The client also takes an arbitrary number of gRPC [dial options](https://pkg.go.dev/google.golang.org/grpc#DialOption) that are passed along to `grpc.Dial()`.

#### Config

`Config{}` has the following properties
| Name | Type | Description |
| :--- | :--- | :--- |
| `Address` | string  | host:port address of ldlm server |
| `Password` |  string  | Password to use for LDLM server |
| `NoAutoRenew`  | bool  |  Don't automatically renew locks in a background goroutine when a lock timeout is specified for a lock |
| `UseTls`    |  bool  |  Use TLS to connect to the server |
| `SkipVerify`  | bool  |  Don't verify the server's certificate |
| `CAFile`   | string |  File containing a CA certificate |
| `TlsCert`  |  string  | File containing a TLS certificate for this client |
| `TlsKey`   |  string  |  File containing a TLS key for this client |
| `MaxRetries`  | int  |  Number of times to retry requests that have failed due to network errors |


### Lock Object

`client.Lock` objects are returned from successful `Lock()` and `TryLock()` client methods. They have the following members:

| Name | Type | Description |
| :--- | :--- | :--- |
| `Name` | `string` | The name of the lock |
| `Key` | `string` | the key for the lock |
| `Locked` | `bool` | whether the was acquired or not |
| `Unlock()` | `func() (error)` | method to unlock the lock |

### Lock Options

Lock operations take a `*LockOptions` object that has the following properties:

| Name | Type | Description |
| :--- | :--- | :--- |
| `LockTimeoutSeconds` | `int32` | The lock timeout. Leave unspecified or set to `0` for no timeout. |
| `WaitTimeoutSeconds` | `int32` | Maximum amount of time to wait to acquire a lock. Leave unspecified or use `0` to wait indefinitely. |
| `Size` | `int32` | The size of the lock. Unspecified will result in a lock with a size of `1`.|


### Lock

`Lock()` attempts to acquire a lock in LDLM. It will block until the lock is acquired or until `WaitTimeoutSeconds` has elapsed (if specified).

`Lock()` accepts the following arguments.

| Type | Description |
|:--- | :--- |
| `string` | Name of the lock to acquire |
| `*LockOptions` | Options for the lock |

It returns a `*Lock` and an `error`.

#### Examples

Simple lock
```go
lock, err = c.Lock("my-task", nil)
if err != nil {
    // handle err
}

doWork("my-task")

lock.Unlock()
```

Wait timeout
```go
lock, err = c.Lock("my-task", &client.LockOptions{WaitTimeoutSeconds: 5})
if err != nil {
    panic(err)
}

if !lock.Locked {
    fmt.Println("Couldn't obtain lock within 5 seconds")
    return
}

doWork("my-task")

lock.Unlock()

```

### Try Lock
`TryLock()` attempts to acquire a lock and immediately returns; whether the lock was acquired or not. You must inspect the returned lock's `Locked` property to determine if it was acquired.

`TryLock()` accepts the folowing arguments.

| Type | Description |
| :--- | :--- |
| `string` | Name of the lock to acquire |
| `*LockOptions` | Options for the lock |

It returns a `*Lock` and an `error`.

#### Examples

Simple try lock
```go
lock, err = c.TryLock("my-task", nil)
if err != nil {
    // handle err
}
if !lock.Locked {
    // Something else is holding the lock
    return
}

doWork("my-task")

lock.Unlock()
```


### Unlock
`Unlock()` unlocks the specified lock and stops any lock renew job that may be associated with the lock. It must be passed the key that was issued when the lock was acquired. Using a different key will result in an error returned from LDLM. Since an `Unlock()` method is available on `Lock` objects returned by the client, calling this directly should not be needed.

`Unlock()` accepts the following arguments.

|  Type | Description |
| :--- | :--- |
| `string` | Name of the lock |
| `string` | Key for the lock |

It returns a `bool` indicating whether or not the lock was unlocked and an `error`.

#### Examples
Simple unlock
```go
unlocked, err := c.Unlock("my_task", lock.key)
```


## License

Apache 2.0; see [`LICENSE`](../LICENSE) for details.

## Contributing

See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for details.

## Disclaimer

This project is not an official Google project. It is not supported by Google and Google specifically disclaims all warranties as to its quality, merchantability, or fitness for a particular purpose.
