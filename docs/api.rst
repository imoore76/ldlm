=============
API Usage
=============

API clients can be created using any language supported by gRPC.
If a :ref:`native client<api:native clients>` is not available for your language,
|examples| in other languages are available.

Native Clients
================

.. |basicauth| raw:: html

    <a href="https://en.wikipedia.org/wiki/Basic_access_authentication" target="_blank">basic auth</a>

.. |golang| raw:: html

    <a href="https://github.com/imoore76/ldlm/tree/main/client" target="_blank">Go</a>

.. |python| raw:: html

    <a href="https://github.com/imoore76/py-ldlm" target="_blank">Python</a>

.. |examples| raw:: html

    <a href="https://github.com/imoore76/ldlm/tree/main/examples" target="_blank">examples</a>

Native LDLM clients are available for

* |golang|
* |python|

Native clients have their own usage documented in their respective repos. 

REST Server API
==========================

The :ref:`REST server<configuration:rest server>` accepts JSON input and has
the following API endpoints. 

.. option:: POST /session

    This creates a session in the LDLM REST server and sets a cookie that **must** be included in subsequent requests.

.. option:: DELETE /session

    Closes your session in the LDLM REST server and releases any resources and locks associated with it. REST sessions idle for more than 10 minutes (default) will be automatically removed, so calling this endpoint is not absolutely necessary.

.. option:: POST /v1/lock

    Behaves like :ref:`concepts:TryLock`, and accepts the following parameters:

    * ``name`` - name of lock
    * ``lock_timeout_seconds`` - lock timeout
    * ``size`` - size of lock

.. option:: POST /v1/unlock
    
    Releases a lock and accepts the following parameters:

    * ``name`` - name of the lock
    * ``key`` - key for the lock

.. option:: POST /v1/renew
    
    Renews a lock and accepts the following parameters:

    * ``name`` - name of the lock
    * ``key`` - key for the lock
    * ``lock_timeout_seconds`` - lock timeout

Example REST Client Usage
------------------------------

The following examples use ``curl`` and its cookie jar feature to maintain the session cookie
across requests. If your REST session has been idle for more than 10m
(:ref:`configurable<configuration:REST Session Timeout (advanced)>`),
your session will expire and all locks you have obtained will be unlocked.

.. important::
    :ref:`Session creation<api:Create a session>` and the resulting session cookie are required in order to use
    the REST endpoints.

Create a session
^^^^^^^^^^^^^^^^^^^^^^
Though the session id is included in the output, it is also set in the
response using ``Set-Cookie``. The cookie's name is `ldlm-session`.

.. code-block:: bash

    user@host ~$ curl -X POST -c cookies.txt http://localhost:8080/session | json_pp
    {
        "session_id" : "590a9b5b8f8440b4b5cdc3429df6e85d"
    }


Obtain a lock
^^^^^^^^^^^^^^^^^^^^^^

.. code-block:: bash

    user@host ~$ curl -c cookies.txt -b cookies.txt http://localhost:8080/v1/lock -d '{"name": "My lock", "lock_timeout_seconds": 120}' | json_pp
    {
        "key" : "15b74bf6-e99a-431b-b3c8-54ffbf5fc4a5",
        "locked" : true,
        "name" : "My lock"
    }


Renew a lock
^^^^^^^^^^^^^^^^^^^^^^

.. code-block:: bash
    
    user@host ~$ curl -c cookies.txt -b cookies.txt http://localhost:8080/v1/renew -d '{"name": "My lock", "lock_timeout_seconds": 120, "key":"15b74bf6-e99a-431b-b3c8-54ffbf5fc4a5"}' | json_pp
    {
        "key" : "15b74bf6-e99a-431b-b3c8-54ffbf5fc4a5",
        "locked" : true,
        "name" : "My lock"
    }


Unlock a lock
^^^^^^^^^^^^^^^^^^^^^^

.. code-block:: bash

    user@host ~$ curl -c cookies.txt -b cookies.txt http://localhost:8080/v1/unlock -d '{"name": "My lock", "key":"15b74bf6-e99a-431b-b3c8-54ffbf5fc4a5"}' | json_pp
    {
        "name" : "My lock",
        "unlocked" : true
    }


Delete session
^^^^^^^^^^^^^^^^^^^^^^

.. code-block:: bash

    user@host ~$ curl -X DELETE -b cookies.txt -c cookies.txt http://localhost:8080/session | json_pp
    {
        "session_id": ""
    }

Authentication
^^^^^^^^^^^^^^^^^^^^^^
If you have set :ref:`configuration:Client Password` on the LDLM server, it will
also apply to the REST
endpoint. The password should be supplied using |basicauth|.

.. code-block:: bash

    user@host ~$ LDLM_AUTH=$(echo -n ':mypassword' | base64) curl -X POST -c cookies.txt -H "Authorization: Basic $LDLM_AUTH" http://localhost:8080/session | json_pp
    {
        "session_id": "e45946cc3a474efc8ab6073918d059a6"
    }

REST API Error Format
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
REST API errors are returned in an ``error`` object.

.. code-block:: bash

    user@host ~$ curl -c cookies.txt -b cookies.txt http://localhost:8080/v1/lock -d '{"name": "My lock", "lock_timeout_seconds": 120, "size": 20}' | json_pp
    {
        "error" : {
            "code" : "LockSizeMismatch",
            "message" : "lock size mismatch"
        },
        "key" : "6e5f8cb8-1661-401f-a4db-a3feef22a0ce",
        "locked" : false,
        "name" : "My lock"
    }


API Errors
================

The following API errors may be returned by LDLM API methods.

.. option:: LockDoesNotExist

    This can occur when attempting to unlock or renew a lock that does not exist.

.. option:: InvalidLockKey

    The key specified in the request is not valid.

.. option:: LockWaitTimeout

    The lock could not be acquired in the ``WaitTimeoutSeconds`` duration specified.
    Native LDLM client implementations swallow this error and instead return a lock
    object that is not locked.

.. option:: LockNotLocked

    This can occur when attempting to renew or unlock a lock that is not locked.

.. option:: LockDoesNotExistOrInvalidKey

    This can occur when renewing a lock using an invalid name or key.

.. option:: LockSizeMismatch

    The size of the lock in the LDLM server does not match the size specified. A
    previous lock request was made with a different size.

.. option:: InvalidLockSize

    The specified size in the lock request is not a valid size (must be > 0).
