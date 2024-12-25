# go-ldlm Client

`go-ldlm/client` is a go library for communicating with an LDLM server (http://github.com/imoore76/go-ldlm).

## Installation

```go
// your go application
include "github.com/imoore76/go-ldlm/client"
```

The just run `go mod tidy`

## Usage

### Create a Client
A client takes a context and a client.Config object. You can Cancel the context to abort a client's operations.

```go
c := client.New(context.Background(), client.Config{
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


### Basic Concepts

Locks in an LDLM server generally live until the client unlocks the lock or disconnects. If a client dies while holding a lock, the disconnection is detected and handled in LDLM by releasing the lock.

Depending on your LDLM server configuration, this feature may be disabled and `LockTimeoutSeconds` would be used to specify the maximum amount of time a lock can remain locked without being renewed. The client will take care of renewing locks in the background for you unless you've specified `NoAutoRenew` in the client's options. Otherwise, you must periodically call `Renew(...)` yourself &lt; the lock timeout interval.

To `Unlock()` or renew a lock, you must use the lock key that was issued from the lock request's response. Using a Lock object's `Unlock()` method takes care of this for you. This is exemplified further in the following sections.

### Lock Object

`client.Lock` objects are returned from successful `Lock()` and `TryLock()` client methods. They have the following members:

| Name | Type | Description |
| :--- | :--- | :--- |
| `Name` | `string` | The name of the lock |
| `Key` | `string` | the key for the lock |
| `Locked` | `bool` | whether the was acquired or not |
| `Unlock()` | `func() (bool, error)` | method to unlock the lock |

### Lock Options

Lock operations take a `*LockOptions` object that has the following properties:

| Name | Type | Description |
| :--- | :--- | :--- |
| `LockTimeoutSeconds` | `int32` | The lock timeout. Leave unspecified or set to `0` for no timeout. |
| `WaitTimeoutSeconds` | `int32` | Maximum amount of time to wait to acquire a lock. Leave unspecified or use `0` to wait indefinitely. |
| `Size` | `int32` | The size of the lock. Unspecified will result in a lock with a size of `1`.|


### Lock

`Lock()` attempts to acquire a lock in LDLM. It will block until the lock is acquired or until `WaitTimeoutSeconds` has elapsed (if specified).  If you have set `WaitTimeoutSeconds` and the lock could not be acquired in that time, the returned error will be `ErrLockWaitTimeout`.

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
if err != nill {
    // handle err
}
defer lock.Unlock()

// Perform task...

```

Wait timeout
```go
lock, err = c.Lock("my-task", &client.LockOptions{WaitTimeoutSeconds: 5})
if err != nill {
    // handle err
    if !errors.Is(err, client.ErrLockWaitTimeout) {
        // The error is not because the wait timeout was exceeded
    }
}
defer lock.Unlock()
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
if err != nill {
    // handle err
}
if !lock.Locked {
    // Something else is holding the lock
    return
}

defer lock.Unlock()
// Do work...

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

### Renew Lock
As explained in [Basic Concepts](#basic-concepts), you may specify a lock timeout using a `LockTimeoutSeconds` argument to any of the `*Lock*()` methods. When you do this, the client will renew the lock in the background without you having to do anything. If, for some reason, you want to disable auto renew (`NoAutoRenew=true` in client Config), you will have to renew the lock before it times out using the `Renew()` method.

It takes the following arguments

| Type | Description |
| :--- | :--- |
| `string` | Name of the lock to acquire |
| `string` | The key for the lock |
| `int32` | The new lock expiration timeout (or the same timeout if you'd like) |

It returns a `*Lock` and an `error`.

#### Examples
```go
lock, err = c.Lock("task1-lock", &client.LockOptions{
    LockTimeoutSeconds: 300,
})


if err != nil {
    // handle err
}

// There was no wait timeout set, so if there was no error, the lock was acquired
defer lock.Unlock()

// do some work, then

if _, err := c.Renew("task1-lock", lock.Key, 300); err != nil {
    panic(err)
}

// do some more work, then

if _, err := c.Renew("task1-lock", lock.Key, 300); err != nil {
    panic(err)
}

// do some more work

```
## Common Patterns

### Primary / Secondary Failover

Using a lock, it is relatively simple to implement primary / secondary (or secondaries) failover by running something similar to the following in each server application: 
```go
lock, err := client.Lock("application-primary")

if err != nil {
    return err
}
 
if !lock.Locked {
    // This should not happen 
    return errors.New("error: lock returned but not locked")
}

log.Info("Became primary. Performing work...")

// Do work. Lock will be unlocked if this process dies.

```

### Task Locking

In some queue / worker patterns it may be necessary to lock tasks while they are being performed to avoid duplicate work. This can be done using try lock:

```go
for {
    workItem := queue.Get()

    lock := client.TryLock(workItem.Name)
    if !lock.Locked {
        log.Infof("Work %s already in progress", workItem.Name)
        continue
    }
    defer lock.Unlock()

    // do work ...
}
```

### Resource Utilization Limiting

In some applications it may be necessary to limit the number of concurrent operations on a resource. This can be implemented using lock size:

```go
// Code in each client to restrict the number of concurrent ElasticSearch operations to 10
lock := client1.Lock("ElasticSearchSlot", &client.LockOptions{Size: 10})

if !lock.Locked {
    return errors.New("error: lock returned but not locked")
}

// Perform ES operation

lock.Unlock()
```

Remember, the size of a lock is set by the first client that obtains the lock. If subsequent calls to a acquire this lock (from this or other clients) use a different size, a `LockSizeMismatch` error will be thrown.

## License

Apache 2.0; see [`LICENSE`](../LICENSE) for details.

## Contributing

See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for details.

## Disclaimer

This project is not an official Google project. It is not supported by Google and Google specifically disclaims all warranties as to its quality, merchantability, or fitness for a particular purpose.
