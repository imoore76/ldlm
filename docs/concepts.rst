=========
Concepts
=========

LDLM is conceptually centered around named 
locks which can be locked and unlocked using an
LDLM client. Generally speaking, a lock that is held (locked) can not be obtained until
it is released (unlocked) by the lock holder.

Locks
=========
A lock in LDLM remains locked until the lock holding client unlocks it or 
disconnects, or its specified timeout is reached without it being renewed.
If an LDLM client dies while holding a lock, the disconnection is detected and handled
in LDLM by
releasing any locks held by the client. This effectively eliminates deadlocks.

Name
----------
A lock is uniquely identified by its name. This is specified when the lock is requested.

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


Size
----------
Locks can have a size (defaults to: 1). This allows for a finite (but greater than 1)
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

Timeout
----------
.. note::
    
    Most users will **not** need to set a timeout for the purpose of mitigating
    deadlocks because client disconnects trigger a release of all locks held by
    the client.

In rare cases where client connections are unreliable, one
could use a lock timeout on all locks
and disable the :ref:`Unlock on Client Disconnect <configuration:No Unlock on Client Disconnect>`
option in the LDLM server. ``LockTimeoutSeconds`` specifies the maximum amount of
time a lock can remain locked without being renewed; if the lock is not renewed in time,
it is released. Unless specifically disabled, LDLM clients will automatically renew
the lock in a background 
thread / task / coroutine (language specific) when a lock timeout is specified.

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
will return immediately whether the lock was acquired or not; the return value is inspected to
determine lock acquisition in this case.

In all cases, a ``Lock`` object is returned. The object is a truthy if locked and falsy if
unlocked. It can also be used to unlock and renew the held lock as you will read about below.

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
In cases where a large number of locks are continually created, lock garbage
collection related settings may need to be tweaked.

:ref:`configuration:Lock Garbage Collection Interval (advanced)` determines how often lock
garbage collection will run. :ref:`configuration:Lock Garbage Collection Idle Duration (advanced)`
determines which locks are considered "idle" based on how long they have been unlocked.
