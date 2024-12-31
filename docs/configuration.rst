================
Configuration
================

Server configuration is facilitated using any combination of
(in order of value precedence)
CLI options, environment
variables, and a configuration file. An option specified on the command line will
override the same option set in an environment variable, which will override
the same option set in a config file.

Options
----------------------------------------------

See available by running

.. code-block:: bash

    $ ldlm-server --help


Configuration file
^^^^^^^^^^^^^^^^^^^^^^^^
Path to :ref:`configuration file<configuration:Config File Syntax>`.

.. option:: -c <file>, --config_file <file>
    
    | Environment ``LDLM_CONFIG_FILE``


.. seealso::

    For syntax and format of the configuration file, see :ref:`configuration:Config File Syntax`

Default Lock Timeout
^^^^^^^^^^^^^^^^^^^^^^^^

The lock timeout applied to all locks loaded from the state file
(if configured) at startup.

.. option:: -d <duration>, --default_lock_timeout <duration>

    | Default ``10m``
    | Environment ``LDLM_DEFAULT_LOCK_TIMEOUT``


Keepalive interval
^^^^^^^^^^^^^^^^^^^^^^^^

The frequency at which to send gRCP keepalive requests to connected clients.

.. option:: -k <duration>, --keepalive_interval <duration>

    | Default ``1m``
    | Environment ``LDLM_KEEPALIVE_INTERVAL``

Keepalive timeout
^^^^^^^^^^^^^^^^^^^^^^^^

The duration to wait for a client to respond to the gRPC keepalive request before considering it
disconnected.

.. option:: -t <duration>, --keepalive_timeout <duration>

    | Default ``10s``
    | Environment ``LDLM_KEEPALIVE_TIMEOUT``


Listen address
^^^^^^^^^^^^^^^^^^^^^^^^

Host and port on which to listen.

.. option:: -l <host:port>, --listen_address <host:port>

    | Default ``localhost:3144``
    | Environment ``LDLM_LISTEN_ADDRESS``


Lock Manager Shards (advanced)
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Number of lock manager shards to use. More *may* increase performance if there is a lot of mutex
contention.

.. option:: --shards <number>

    | Default ``16``
    | Environment ``LDLM_SHARDS``


Lock Garbage Collection Interval (advanced)
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

How often to perform garbage collection (deletion) of idle locks.

.. option:: -g <duration>, --lock_gc_interval <duration>

    | Default ``30m``
    | Environment ``LDLM_LOCK_GC_INTERVAL``


Lock Garbage Collection Idle Duration (advanced)
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Minimum time a lock has to be idle (unlocked, without lock attempts) before being considered for
garbage collection.

.. option:: -m <duration>, --lock_gc_min_idle <duration>

    | Default ``5m``
    | Environment ``LDLM_LOCK_GC_MIN_IDLE``


Log Level
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Log level of the server. Must be one of:

* ``debug``
* ``info``
* ``warn``
* ``error``

.. option:: -v <level>, --log_level <level>

    | Default ``info``
    | Environment ``LDLM_LOG_LEVEL``


IPC Socket File
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Path to a file to use for IPC (inter process communication) 
with the ``ldlm-lock`` :ref:`command<usage:Lock Tool>`.
This file should not exist; it will be created by the
server. Set to an empty string to disable IPC. 

.. option:: --ipc_socket_file <file path>

    | Default *platform dependent path*
    | Environment ``LDLM_IPC_SOCKET_FILE``


State File
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

The file in which in which to store lock state each time a locking or unlocking operation is
performed. Specify if you want LDLM to maintain locks across restarts.

.. option:: -s <file path>, --state_file <file path>

    | Environment ``LDLM_STATE_FILE``


No Unlock on Client Disconnect
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Disable the default behavior of clearing locks held by clients when a client disconnect is
detected.

.. option:: -n, --no_clear_on_disconnect

    | Environment ``LDLM_NO_CLEAR_ON_DISCONNECT``


TLS Certificate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Path to TLS certificate file to enable LDLM server TLS.

.. option:: --tls_cert <file>

    | Environment ``LDLM_TLS_CERT``


TLS Certificate Key
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Path to server TLS certificate key file.

.. option:: --tls_key <file>

    | Environment ``LDLM_TLS_KEY``


Verify TLS Client Certificates
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Require and verify TLS certificates of clients.

.. option:: --client_cert_verify

    | Environment ``LDLM_CLIENT_CERT_VERIFY``


Client CA Certificate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Path to a file containing client CA's (certificate authority) certificate.
Setting this will automatically enable client certificate verification.

.. option:: --client_ca <file>

    | Environment ``LDLM_CLIENT_CA``

Client Password
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Require clients to specify this password.

.. option:: --password

    | Environment ``LDLM_PASSWORD``


REST Listen Address
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

The host:port on which the REST server should listen.
Leave empty to disable the REST server. Default is empty.

.. option:: -r <host:port>, --rest_listen_address <host:port>

    | Environment ``LDLM_REST_LISTEN_ADDRESS``


REST Session Timeout (advanced)
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

The duration a REST session can be idle before it is considered invalid.

.. option:: --rest_session_timeout <duration>

    | Default ``10m``
    | Environment ``LDLM_REST_SESSION_TIMEOUT``


Environment Variables
-------------------------

Configuration from environment variables consists of setting ``LDLM_<upper case cli flag>``. For example

.. code-block:: bash

    LDLM_LISTEN_ADDRESS=0.0.0.0:3144
    LDLM_PASSWORD=mysecret
    LDLM_LOG_LEVEL=info

You can see the environment variables for all
configuration options by running

.. code-block:: bash

    $ ldlm-server --print_env_template

.. hint::

    The output of the above command can be used to generate a ``.env`` file.

Environment variable names are also documented in each configuration option.

Config File Syntax
---------------------

YAML and JSON file formats are supported.
The configuration file specified must end in ``.json``, ``.yaml``, or ``.yml``.

Configuration options are the same as the CLI flag names and function in exactly
the same way. For example

.. code-block:: yaml
    :caption: YAML

    listen_address: "0.0.0.0:2000"
    lock_gc_interval: "20m"
    lock_gc_min_idle: "10m"
    log_level: info

.. code-block:: json
    :caption: JSON

    {
        "listen_address": "0.0.0.0:6000",
        "lock_gc_interval":"20m"
    }

You can generate an example yaml configuration file with all of LDLM's supported
configuration options by running

.. code-block:: bash

    $ ldlm-server --print_yaml_template >ldlm-config.yaml

.. seealso::

    How to :ref:`specify a configuration file <configuration:Configuration file>`.

Configuration Recipes
------------------------

Server TLS
^^^^^^^^^^^^^^^^^

Enable server TLS by specifying :ref:`configuration:TLS Certificate` and
:ref:`configuration:TLS Certificate Key`. E.g.

.. code-block:: text

    ldlm-server --tls_cert <cert_file_location> --tls_key <key_file_location>


The server startup logs should indicate that TLS is enabled

.. code-block:: text
    
    {"time":"2024-04-03T18:15:04.723958-04:00","level":"INFO","msg":"Loaded TLS configuration"}
    {"time":"2024-04-03T18:15:04.724002-04:00","level":"INFO","msg":"gRPC server started. Listening on localhost:3144"}


If the LDLM server certificate is signed using an internal CA, you may need to include
the CA cert with your LDLM clients; each client implementation has 
a ``CA Certificate`` option.

Mutual TLS
^^^^^^^^^^^^^^^^^

To enable client TLS certificate verification, use the :ref:`configuration:Verify TLS Client Certificates`
option.

If the CA that issued the client certs is not in a path searched by GO, you must specify
the path to the CA cert with  :ref:`configuration:Client CA Certificate`.
These options should be combined with :ref:`configuration:Server TLS` options.

.. note::
    Specifying the :ref:`configuration:Client CA Certificate` will automatically
    enable client cert verification,
    so specifying :ref:`configuration:Verify TLS Client Certificates`
    would not be needed in those cases.

REST Server
^^^^^^^^^^^^^^^^^

LDLM's REST server can be enabled by simply specifying the :ref:`configuration:REST Listen Address`.
Left unspecified, its REST server will not be enabled.
See :ref:`REST Endpoint API usage<api:REST Server API>`.

.. note::
    
    There is no way to disable LDLM's gRPC server, but you can specify its
    :ref:`configuration:Listen address` to be
    the local loopback address (``127.0.0.1``) so that it is not exposed.
