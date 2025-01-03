=========
Concepts
=========

LDLM is conceptually centered around named 
locks which can be locked and unlocked using an
LDLM client. Generally speaking, a lock that is held (locked) can not be obtained until
it is released (unlocked) by the lock holder.

.. note::

    The examples that follow use client libraries available
    :ref:`here<api:Native Clients>`.

Locks
=========
A lock in LDLM remains locked until the lock holding client unlocks it or 
disconnects, or its specified timeout is reached without it being renewed.
If an LDLM client dies while holding a lock, the disconnection is detected and handled
in LDLM by
releasing any locks held by the client. This effectively eliminates deadlocks.

Lock Name
----------
A lock is uniquely identified by the name specified when the lock is requested.
There are no character restrictions on lock names, but it is recommended to use
a name that is unique to the task or resource being locked otherwise locking would
be quite useless.

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            client = ldlm.Client("ldlm-server:3144")

            lock = client.lock("my-task")

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"            

            c, err := client.New(context.Background(), client.Config{
                Address: "localhost:3144",
            })

            if err != nil {
                panic(err)
            }

            lock, err = c.Lock("my-task", nil)


Lock Size
----------
Locks can have a size (defaults to: 1). This allows for a finite, but greater than 1
number of lock acquisitions to be held on the same lock.

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            # Number of expensive operation slots
            ES_SLOTS = 20

            client = ldlm.Client("ldlm-server:3144")

            lock = client.lock("expensive_operation", size=ES_SLOTS)

            # Do operation

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"

            const ES_SLOTS = 10

            c, err := client.New(context.Background(), client.Config{
                Address: "localhost:3144",
            })

            if err != nil {
                panic(err)
            }

            lock, err := c.Lock("expensive_operation", &client.LockOptions{
                Size: ES_SLOTS,
            })

Lock Timeout
-------------
When acquiring a lock, a lock timeout specifies the maximum amount of
time a lock can remain locked without
being renewed; if the lock is not renewed in time, it is released. Unless specifically disabled,
LDLM clients will automatically renew the lock in a background 
thread / task / coroutine (language specific) when a lock timeout is specified.

Using lock timeouts can be useful for implementing a :ref:`client side<uses:Client-side Rate Limiting>`
or :ref:`server side<uses:Server-side Rate Limiting>` rate limiter.

.. note::
    
    In rare cases where client connections are unreliable,
    a lock timeout could be used on all locks
    and the :ref:`No Unlock on Client Disconnect <configuration:No Unlock on Client Disconnect>`
    option set in the LDLM server. This would be tolerant of client disconnects
    while still ensuring that no deadlocks occur.
    
    In most most cases, it is recommended to leave the default behavior which
    releases locks when a client unexpectedly quits and its connection drops.

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            client = ldlm.Client("ldlm-server:3144")

            lock = client.lock("my-task", lock_timeout_seconds=300)

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"

            c, err := client.New(context.Background(), client.Config{
                Address: "localhost:3144",
            })

            if err != nil {
                panic(err)
            }

            lock, err := c.Lock("expensive_operation", &client.LockOptions{
                LockTimeoutSeconds: 300,
            })


Acquiring a Lock
===========================

Locks are generally acquired using ``Lock()`` or ``TryLock()``. ``Lock()`` will block until
the lock is acquired or until ``WaitTimeoutSeconds`` have elapsed (if specified). ``TryLock()``
will return immediately whether the lock was acquired or not.

In all cases, a ``Lock`` object is returned. This object can be inspected (``.Locked`` property)
to determine if the lock was acquired and can be released using the ``Unlock()`` method.

.. note::

    When using ``Lock()`` without a wait timeout set, the client will block until the lock is acquired.
    There is no need to check the ``Locked`` property of the returned ``Lock`` object.


Examples
----------

Simple lock
^^^^^^^^^^^^^^

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            # Block until lock is obtained
            lock = client.lock("my-task")

            # Do work, then release lock
            lock.unlock()

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"            

            c, err := client.New(context.Background(), client.Config{
                Address: "localhost:3144",
            })
            if err != nil {
                panic(err)
            }

            // Block until a lock is obtained
            lock, err := c.Lock("my-lock", nil)

            if err != nil {
                panic(err)
            }

            // Do some work

            if err = lock.Unlock(); err != nil {
                panic(err)
            }

Wait timeout
^^^^^^^^^^^^^^
..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            # Wait at most 30 seconds to acquire lock
            lock = client.lock("my-task", wait_timeout_seconds=30)
            if not lock:
                print("Could not obtain lock within 30 seconds.")
                return
            # Do work, then release lock
            lock.unlock()

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"            

            c, err := client.New(context.Background(), client.Config{
                Address: "localhost:3144",
            })

            if err != nil {
                panic(err)
            }

            lock, err := c.Lock("my-lock", &client.LockOptions{
                WaitTimeoutSeconds: 30,
            })

            if err != nil {
                panic(err)
            }

            // Check lock
            if !lock.Locked {
                fmt.Println("Failed to acquire lock after 30 seconds")
                return
            }

            // Do work

            if err = lock.Unlock(); err != nil {
                panic(err)
            }

TryLock
^^^^^^^^^^^^
..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            # This is non-blocking
            lock = client.try_lock("my-task")
            if not lock:
                print("Lock already acquired.")
                return
            # Do work, then release lock
            lock.unlock()

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"            

            c, err := client.New(context.Background(), client.Config{
                Address: "localhost:3144",
            })

            if err != nil {
                panic(err)
            }
            lock, err := c.TryLock("my-lock", nil)

            if err != nil {
                panic(err)
            }

            // Check lock
            if !lock.Locked {
                fmt.Println("Failed to acquire lock")
                return
            }

            // Do work

            if err = lock.Unlock(); err != nil {
                panic(err)
            }

Releasing a lock
==================
The ``Unlock()`` method is used to release a held lock.

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            client = ldlm.Client("ldlm-server:3144")

            lock = client.lock("my-task")

            # Do task

            lock.unlock()

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"            

            c, err := client.New(context.Background(), client.Config{
                Address: "localhost:3144",
            })

            if err != nil {
                panic(err)
            }
            lock, err := c.Lock("my-lock", nil)

            if err != nil {
                panic(err)
            }

            // Do work

            if err = lock.Unlock(); err != nil {
                panic(err)
            }

Manually Renewing a lock
===========================

.. important::
    
    Most users will not need to worry about lock renewal.

If you have a very specific use case where you have disabled automatic lock renewal in the
LDLM client being used, manually renewing a lock can be done by calling ``Renew()`` on
the ``Lock`` object returned by any locking function.

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            client = ldlm.Client("ldlm-server:3144")

            lock = client.lock("my-task")

            # Do work

            lock.renew(300)

            # Do more work

            lock.renew(300)

            # Do more work

            lock.unlock()

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"            

            c, err := client.New(context.Background(), client.Config{
                Address: "localhost:3144",
            })

            if err != nil {
                panic(err)
            }
            lock, err := c.Lock("my-lock", nil)

            if err != nil {
                panic(err)
            }

            // Do work

            if err = lock.Renew(300); err != nil {
                panic(err)
            }

            // Do more work

            if err = lock.Renew(300); err != nil {
                panic(err)
            }

            // Do more work

            if err = lock.Unlock(); err != nil {
                panic(err)
            }


Advanced
==========================

Lock Keys
--------------
Internally, LDLM manages client synchronization using lock keys. If a client attempts
to ``Unlock()`` a lock that it no longer has acquired (either via timeout, stateless server
restart, or network disconnect), an error is returned.

Lock keys are meant to detect when LDLM and a client are out of sync.
They are not cryptographic. They are not secret. They are not meant to deter malicious
users from releasing locks.

When desynchronization occurs and an incorrect key is used, an 
:ref:`InvalidLockKey<api:api errors>`
error is returned or raised (language specific) by the ``Unlock()`` method.

Lock Garbage Collection
----------------------------
Each lock requires a small, but non-zero amount of memory.
For performance reasons, "idle" (unlocked) locks in LDLM live until an internal lock
garbage collection task runs.
In cases where a large number of locks are continually created
at a high rate, lock garbage collection related settings may need to be adjusted.

:ref:`configuration:Lock Garbage Collection Interval (advanced)` determines how often lock
garbage collection will run. :ref:`configuration:Lock Garbage Collection Idle Duration (advanced)`
determines which locks are considered "idle" based on how long they have been unlocked.
