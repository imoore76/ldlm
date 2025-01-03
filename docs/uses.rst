
===============
Use Cases
===============

Below are some common use cases and client code examples.

.. note::

    The examples that follow use client libraries available
    :ref:`here<api:Native Clients>`.

Failover
===============================

Implement primary / backup
failover by running something similar to the following in each server application:

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            client = ldlm.Client("ldlm-server:3144")

            # This will block until lock is acquired
            lock = client.lock("application-primary")

            logger.info("Became primary. Performing work...")

            # Lock will be unlocked when this process ends.

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"

            c, _ := client.New(context.Background(), client.Config{
                Address: "ldlm-server:3144",
            })

            // This will block until the lock is acquired
            lock, err := c.Lock("application-primary", nil)

            if err != nil {
                panic(err)
            }

            fmt.Println("Became primary. Performing work...")

            // Lock will be unlocked when this process ends.

Clustered Failover
===============================

Implement a failover cluster of servers
by running something similar to the following in each server application:

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            # Allow 10 servers active at a time
            ALLOW_ACTIVE = 10

            client = ldlm.Client("ldlm-server:3144")

            # This will block until lock is acquired
            lock = client.lock("application-cluster", size=ALLOW_ACTIVE)

            logger.info("Became active. Performing work...")

            # Lock will be unlocked when this process ends.

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"

            // Allow 10 servers active at a time
            const allowActive = 10

            c, _ := client.New(context.Background(), client.Config{
                Address: "ldlm-server:3144",
            })

            // This will block until the lock is acquired
            lock, err := c.Lock("application-cluster", &&client.LockOptions{
                Size: allowActive,
            })

            if err != nil {
                panic(err)
            }

            fmt.Println("Became active. Performing work...")

            // Lock will be unlocked when this process ends.


Task Locking
===============================

In some queue / worker patterns it may be necessary to lock tasks while they are
being performed to avoid duplicate work. This can be done using try lock:

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            client = ldlm.Client("ldlm-server:3144")

            while True:

                work_item = queue.Get()

                lock = client.try_lock(work_item.name)
                if not lock:
                    log.debug(f"Work {work_item.name} already in progress")
                    continue

                try:
                    run_job(work_item)
                finally:
                    lock.unlock()

    .. group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"

            c, _ := client.New(context.Background(), client.Config{
                Address: "ldlm-server:3144",
            })

            for {
                workItem := queue.Get()

                lock, err := client.TryLock(workItem.Name)

                if (err) {
                    fmt.Printf("Error locking work: %w", err)
                    continue
                }

                if !lock.Locked {
                    log.Infof("Work %s already in progress", workItem.Name)
                    continue
                }

                func() {
                    defer lock.Unlock()
                    RunJob(workItem)
                }()
            }

Resource Utilization Limiting
===============================

In some applications it may be necessary to limit the number of concurrent operations on a
resource. Assuming distributed clients sharing the same codebase, (e.g. deployed kubernetes pods)
this can be implemented using lock size.

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            ES_SLOTS = 10

            # Code in each client to restrict the number of concurrent ElasticSearch operations to 10
            client = ldlm.Client("ldlm-server:3144")

            # Block until a slot becomes available.
            lock = client.lock("ElasticSearchSlot", size=ES_SLOTS):

            try:
                elastic_search.do_something()
            finally:
                lock.unlock()

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"

            const elasticSearchSlots = 10

            c, _ := client.New(context.Background(), client.Config{
                Address: "ldlm-server:3144",
            })

            // This will block until the lock is acquired
            lock, err := c.Lock("ElasticSearchSlot", &&client.LockOptions{
                Size: elasticSearchSlots,
            })

            if err != nil {
                panic(err)
            }

            func() {
                defer lock.Unlock()
                ElasticSearch.DoSomething()
            }()


Client-side Rate Limiting
===============================

Limit request rate to a service using locks. Like the task locking example, this assumes
distributed clients sharing the same codebase, (e.g. deployed kubernetes pods).

.. important::

    Automatic lock renewal must be disabled in the LDLM client for the 
    rate limiting recipe to function properly. This is demonstrated in
    the client instantiation code in the examples.


..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            # Allow 30 requests every 60 seconds
            RATE_LIMIT_SIZE = 30
            RATE_LIMIT_SECONDS = 60

            # A client-enforced sliding window of 30 requests per minute.
            client = ldlm.Client("ldlm-server:3144", auto_renew_locks=False)

            # This will block until lock is acquired.
            client.lock(
                "RateLimitExpensiveService",
                size=RATE_LIMIT_SIZE,
                lock_timeout_seconds=RATE_LIMIT_SECONDS
            )

            results = expensive_service.query("getAll")
            
            # Do not unlock. Lock will expire in 60 seconds, which enforces the rate window.

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"

            const (
                rateLimitSize    = 30
                rateLimitSeconds = 60
            )

            c, _ := client.New(context.Background(), client.Config{
                Address: "ldlm-server:3144",
                NoAutoRenew: true,
            })

            // This will block until the lock is acquired
            err := c.Lock("RateLimitExpensiveService", &&client.LockOptions{
                Size: rateLimitSize,
                LockTimeoutSeconds: rateLimitSeconds,
            })

            if err != nil {
                panic(err)
            }

            results = expensive_service.query("getAll")

            // Do not unlock. Lock will expire in 60 seconds, which enforces the rate window.
            

Server-side Rate Limiting
===============================

.. important::

    Automatic lock renewal must be disabled in the LDLM client for the 
    rate limiting recipe to function properly. This is demonstrated in
    the client instantiation code in the examples.

Limit request rate to a service using locks:

..  tabs::

    ..  group-tab:: Python

        .. code-block:: python

            import ldlm

            # Allow 30 requests every 60 seconds
            RATE_LIMIT_SIZE = 30
            RATE_LIMIT_SECONDS = 60

            client = ldlm.Client("ldlm-server:3144", auto_renew_locks=False)

            def generate_image(request):
                """Request handler for expensive AI image generation"""

                lock = client.try_lock(
                    "generate_image",
                    size=RATE_LIMIT_SIZE,
                    lock_timeout_seconds=RATE_LIMIT_SECONDS
                )

                if not lock:
                    return HttpResponse("Too Many Requests", status=429)

                # Generate image.
                for chunk in ai_image_generator(request)
                    yield chunk

                # Do not unlock. Lock will expire in 60 seconds, which enforces the rate window.

    ..  group-tab:: Go

        .. code-block:: go

            import "github.com/imoore76/ldlm/client"

            const (
                rateLimitSize    = 30
                rateLimitSeconds = 60
            )

            c, _ := client.New(context.Background(), client.Config{
                Address: "ldlm-server:3144",
                NoAutoRenew: true,
            })

            func aiImageGenerator(w http.ResponseWriter, r *http.Request) {
                // Process the request

                lock, err := c.TryLock("GenerateAIImage", &&client.LockOptions{
                    Size: rateLimitSize,
                    LockTimeoutSeconds: rateLimitSeconds,
                })

                if err != nil {
                    panic(err)
                }

                if !lock.Locked {
                    w.WriteHeader(http.StatusTooManyRequests) // 429
                    w.Write([]byte("Too Many Requests"))
                    return
                }

                generateAIImage(w, r)

                // Do not unlock. Lock will expire in 60 seconds, which enforces the rate window.
            }
