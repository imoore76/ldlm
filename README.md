# ldlm

ldlm is a lightweight distributed lock manager implemented over gRPC.

## Installation

Download and install the latest release from [github](https://github.com/imoore76/go-ldlm/releases/latest) for your platform. Packages for linux distributions are also available there.

For containerized environments, the docker image `ian76/ldlm:latest` is available from [dockerhub](https://hub.docker.com/r/ian76/ldlm).
```
user@host ~$ docker run -p 3144:3144 ian76/ldlm:latest
{"time":"2024-04-27T03:33:03.434075592Z","level":"INFO","msg":"loadState() loaded 0 client locks from state file"}
{"time":"2024-04-27T03:33:03.434286717Z","level":"INFO","msg":"IPC server started","socket":"/tmp/ldlm-ipc.sock"}
{"time":"2024-04-27T03:33:03.434402133Z","level":"WARN","msg":"gRPC server started. Listening on 0.0.0.0:3144"}
```

## Server Usage

    ldlm-server -help

-------------------

| Short | Long                       | Default                         | Description                                                                                                                                                                                    |
|:----- |:-------------------------- |:-------------------------------:|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-c`  | `--config_file`            |                                 | Path to [configuration file](#configuration-file-format).                                                                                                                                      |
| `-d`  | `--default_lock_timeout`   | 10m                             | The lock timeout applied to all locks loaded from the state file (if configured) at startup                                                                                                    |
| `-k`  | `--keepalive_interval`     | 1m                              | The frequency at which to send gRCP keepalive requests to connected clients.                                                                                                                   |
| `-t`  | `--keepalive_timeout`      | 10s                             | The time to wait for a client to respond to the gRPC keepalive request before considering it dead. This will clear all locks held by the client unless `no_clear_on_disconnect` is also set.   |
| `-l`  | `--listen_address`         | localhost:3144                  | Host and port on which to listen.                                                                                                                                                              |
| `-g`  | `--lock_gc_interval`       | 30m                             | How often to perform garbage collection (deletion) of idle locks.                                                                                                                              |
| `-m`  | `--lock_gc_min_idle`       | 5m                              | Minimum time a lock has to be idle (no unlocks or locks) before being considered for garbage collection. Only unlocked locks can be garbage collected.                                         |
| `-v`  | `--log_level`              | info                            | Log level of the server. debug, info, warn, or error                                                                                                                                           |
|       | `--ipc_socket_file`        | &lt;platform dependent path&gt; | Path to the IPC socket file used for communication with the `ldlm-lock` command. Set to an empty string to disable IPC.                                                                        |
| `-s`  | `--state_file`             |                                 | The file in which in which to store lock state each time a locking or unlocking operation is performed. If you want `ldlm` to maintain locks across restarts, point this at a persistent file. |
| `-n`  | `--no_clear_on_disconnect` |                                 | Disable the default behavior of clearing locks held by clients when a client disconnect is detected.                                                                                           |
|       | `--client_cert_verify`     |                                 | Require and verify TLS certificates of clients                                                                                                                                                 |
|       | `--client_ca`              |                                 | Path to a file containing client CA's certificate. Setting this will automatically set `--client_cert_verify`.                                                                                 |
|       | `--password`               |                                 | Require clients to specify this [password](#password). Clients do this by setting the [metadata](https://grpc.io/docs/guides/metadata/) `authorization` key.                                   |
|       | `--tls_cert`               |                                 | Path to TLS certificate file to enable TLS                                                                                                                                                     |
|       | `--tls_key`                |                                 | Path to TLS key file                                                                                                                                                                           |

### Environment Variables

Configuration from environment variables consists of setting `LDLM_<upper case cli flag>`. For example

    LDLM_LISTEN_ADDRESS=0.0.0.0:3144
    LDLM_PASSWORD=mysecret
    LDLM_LOG_LEVEL=info

### Configuration File Format

Yaml and JSON file formats are supported. The configuration file specified must end in `.json`, `.yaml`, or `.yml`.

Configuration options are the same as the CLI flag names and function in exactly the same way. For example

yaml

```yaml
# ldlm_config.yaml
listen_address: "0.0.0.0:2000"
lock_gc_interval: "20m"
lock_gc_min_idle: "10m"
log_level: info
```

json

```json
{
   "listen_address": "0.0.0.0:6000",
   "lock_gc_interval":"20m"
}
```

## API Usage

Basic client usage consists of locking and unlocking locks named by the API client. When a lock is obtained, the response contains a `key` that must be used to unlock the lock - it can not be unlocked using any other [key](#lock-keys). 

The API functions are `Lock`, `TryLock`, `Unlock`, and `RefreshLock`. Here are some examples in a language I've completely invented for the purpose of this demonstration.

```javascript
resp = client.Lock({
    Name: "work-item1",
})

if (resp.Error) {
    // handle error
    print(resp.Error.Message)
    return
}

if (!resp.Locked) {
    print("Could not obtain lock")
    return
}

//
// Do work...
//
RunJob(workItem)

resp = client.Unlock({
    Name: resp.Name,
    Key: resp.Key,
})

if (resp.Error) {
    // handle error
    print(resp.Error.Message)
    return
}
```

If you do not want the client to wait to acquire a lock, use `TryLock()` which will return immediately. 

### Lock Options

#### WaitTimeoutSeconds

Only available on `Lock()` since `TryLock()` does not wait to acquire a lock. The number of seconds to wait to acquire a lock.

```javascript
resp = client.Lock({
    Name: "work-item1",
    WaitTimeoutSeconds: 30,
})

if (resp.Error) {
    // handle error. If wait timed out, resp.Error.Code will be LockWaitTimeout
    print(resp.Error.Message)
    return
}

if (!resp.Locked) {
    print("Could not obtain lock")
    return
}
```

#### LockTimeoutSeconds

The number of seconds before a lock will be automatically unlocked if not refreshed. 

```javascript
resp = client.Lock({
    Name: "work-item1",
    LockTimeoutSeconds: 300,
})

if (!resp.Locked) {
    return
}

refresher = spawn(function() {
    while (true) {
        sleep(240)
        client.RefreshLock({
            Name: resp.Name,
            Key: resp.Key,
            LockTimeoutSeconds: 300,
        })
    }
})

//
// Do work...
//
RunLongJob(workItem)

refresher.stop()

resp = client.Unlock({
        Name: resp.Name,
        Key: resp.Key,
})
```

## Lock Keys

Lock keys are meant to detect when ldlm and a client are out of sync. They are not cryptographic. They are not secret. They are not meant to deter malicious users from releasing locks.

## ldlm-lock commands

The ldlm-lock command is used to manipulate locks in a running ldlm server on the CLI. See also `ldlm-lock -help`.

### List Locks

This prints a list of locks and their keys

    ldlm-lock list

### Force Unlocking

If a lock ever becomes deadlocked (this *should* not happen), you can unlock it by running the following on the same machine as a running ldlm server:

    ldlm-lock unlock <lock name>

## Examples

### Password

To require a password of connecting clients, use the `--password` option to `ldlm`. Optionally set it as an environment variable `LDLM_PASSWORD` or in a config file before running the server instead of having it visible in the process list.

The password must be specified an `authorization` key in the [metadata](https://grpc.io/docs/guides/metadata/) of client requests.

Go client

```go
// Add authorization metadata to context
ctx = metadata.AppendToOutgoingContext(
    context.Background(), "authorization", "secret",
)

resp, err := client.Lock(ctx, &pb.LockRequest{
    Name: lockName,
})

if err != nil {
    panic(err)
}
```

python client

```python
import grpc
from protos import ldlm_pb2 as pb2
from protos import ldlm_pb2_grpc as pb2grpc

chan = grpc.insecure_channel("localhost:3144")
stub = pb2grpc.LDLMStub(chan)

resp = stub.TryLock(
    pb2.TryLockRequest(name="work-item1"),
    # authorization metadata
    metadata=(("authorization", "secret"),),
)
```

### Server TLS

Enable server TLS by specifying `--tls_cert` and `--tls_key`. E.g.

```
ldlm-server --tls_cert <cert_file_location> --tls_key <key_file_location>
```

The server startup logs should indicate that TLS is enabled

```
{"time":"2024-04-03T18:15:04.723958-04:00","level":"INFO","msg":"Loaded TLS configuration"}
{"time":"2024-04-03T18:15:04.724002-04:00","level":"INFO","msg":"gRPC server started. Listening on localhost:3144"}
```

### Mutual TLS

To enable client TLS certificate verification, use `--client_cert_verify`. If the CA that issued the client certs is not in a path searched by GO, you may also specify the path to the CA cert with `--client_ca`. These options should be combined with Server TLS options.

```
ldlm-server --tls_cert <cert_file_location> --tls_key <key_file_location> --client_ca <client ca cert file location>
```

Specifying the client CA (`--client_ca`) will automatically enable client cert verification, so specifying `--client_cert_verify` is not needed in those cases.

Go client

```go
import (
    "crypto/tls"
    "os"
    "crypto/x509"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"

    pb "github.com/imoore76/go-ldlm/protos"
)

tlsc := &tls.Config{}
if cc, err := tls.LoadX509KeyPair("/certs/cert.pem", "/certs/key.pem"); err != nil {
    panic(err)
} else {
    tlsc.Certificates = []tls.Certificate{cc}
}

if cacert, err := os.ReadFile("/certs/ca_cert.pem"); err != nil {
    panic("Failed to read CA certificate: " + err.Error())
} else {
    certPool := x509.NewCertPool()
    if !certPool.AppendCertsFromPEM(cacert) {
        panic("failed to add CA certificate")
    }
    tlsc.RootCAs = certPool
}

creds := credentials.NewTLS(tlsc)

conn, err := grpc.Dial(
    "localhost:3144",
    grpc.WithTransportCredentials(creds),
)
if err != nil {
    panic(err)
}

client := pb.NewLDLMClient(conn)
```

python client

```python
import grpc
from protos import ldlm_pb2 as pb2
from protos import ldlm_pb2_grpc as pb2grpc

CLIENT_CERT = "/certs/client_cert.pem"
CLIENT_KEY = "/certs/client_key.pem"
CA_CERT= "/certs/ca_cert.pem"


def readfile(fl):
   with open(fl, 'rb') as f:
      return f.read()


creds = grpc.ssl_channel_credentials(
   readfile(CA_CERT),
   private_key=readfile(CLIENT_KEY),
   certificate_chain=readfile(CLIENT_CERT),
)

chan = grpc.secure_channel("localhost:3144", creds)
stub = pb2grpc.LDLMStub(chan)

resp = stub.TryLock(
    pb2.TryLockRequest(name="work-item1"),
)

print(resp)
```

### Example Clients

See example code for ldlm clients in the <a href="./examples">examples</a> folder.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for details.

## License

Apache 2.0; see [`LICENSE`](LICENSE) for details.

## Disclaimer

This project is not an official Google project. It is not supported by
Google and Google specifically disclaims all warranties as to its quality,
merchantability, or fitness for a particular purpose.
