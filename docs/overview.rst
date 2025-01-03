=============
Overview
=============

LDLM is a lightweight distributed lock manager with many use cases.

Installation
=============

Numerous installation options exist for LDLM.

.. |github_releases| raw:: html

    <a href="https://github.com/imoore76/ldlm/releases/latest" target="_blank">github releases</a>

.. |dockerhub| raw:: html

    <a href="https://hub.docker.com/r/ian76/ldlm" target="_blank">dockerhub</a>

Binary and Package Installation
------------------------------------
Packages are available as

* RPM package
* DEB package
* Generic Linux tgz package
* Mac OSX (darwin)
* Windows zip

on the LDLM |github_releases| page.

Container Image (docker, kubernetes, et al.)
---------------------------------------------------------
For containerized environments, the docker image ``ian76/ldlm:latest`` is available from |dockerhub|.

.. code-block:: text

    user@host ~$ docker run -p 3144:3144 ian76/ldlm:latest
    {"time":"2024-04-27T03:33:03.434075592Z","level":"INFO","msg":"loadState() loaded 0 client locks from state file"}
    {"time":"2024-04-27T03:33:03.434286717Z","level":"INFO","msg":"IPC server started","socket":"/tmp/ldlm-ipc.sock"}
    {"time":"2024-04-27T03:33:03.434402133Z","level":"WARN","msg":"gRPC server started. Listening on 0.0.0.0:3144"}

.. seealso::

    Clients for LDLM can be find in :doc:`API Usage > Native Clients </api>`
