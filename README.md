<p align="center">
<img src="images/LDLM%20Logo%20Symbol.png" height="95" width="100" alt="ldlm logo" />
</p>

# LDLM

LDLM is a **L**ightweight **D**istributed **L**ock **M**anager implemented over gRPC and REST.

## Installation

Download and install the latest release from [github](https://github.com/imoore76/ldlm/releases/latest) for your platform. Packages for linux distributions are also available there.

For containerized environments, the docker image `ian76/ldlm:latest` is available from [dockerhub](https://hub.docker.com/r/ian76/ldlm).
```
user@host ~$ docker run -p 3144:3144 ian76/ldlm:latest
{"time":"2024-04-27T03:33:03.434075592Z","level":"INFO","msg":"loadState() loaded 0 client locks from state file"}
{"time":"2024-04-27T03:33:03.434286717Z","level":"INFO","msg":"IPC server started","socket":"/tmp/ldlm-ipc.sock"}
{"time":"2024-04-27T03:33:03.434402133Z","level":"WARN","msg":"gRPC server started. Listening on 0.0.0.0:3144"}
```

## Usage

Full documentation is available at http://ldlm.readthedocs.io/

## Clients

Native LDLM clients which have their own usage documented
in their respective repos are available for

* <a href="https://github.com/imoore76/ldlm/tree/main/client" target="_blank">Go</a>
* <a href="https://github.com/imoore76/py-ldlm" target="_blank">Python</a>

API clients can be created using any language supported by gRPC.
If a native client is not available for your language,
<a href="https://github.com/imoore76/ldlm/tree/main/examples" target="_blank">examples</a>
for other languages are available.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for details.

## License

Apache 2.0; see [`LICENSE`](LICENSE) for details.

## Disclaimer

This project is not an official Google project. It is not supported by
Google and Google specifically disclaims all warranties as to its quality,
merchantability, or fitness for a particular purpose.
