#!/usr/bin/python
#
# Copyright 2024 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""

python3 -m grpc_tools.protoc -I../../ --python_out=./protos --grpc_python_out=./protos ../../ldlm.proto

"""
import time
import os
import sys
from contextlib import contextmanager

import grpc
from grpc._channel import _InactiveRpcError
import exceptions

SCRIPT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "protos")
sys.path.append(SCRIPT_DIR)

from protos import ldlm_pb2 as pb2
from protos import ldlm_pb2_grpc as pb2grpc

from threading import Timer


CLIENT_CERT = "../../testcerts/client_cert.pem"
CLIENT_KEY = "../../testcerts/client_key.pem"
CA_CERT = "../../testcerts/ca_cert.pem"


class RenewLockTimer(Timer):
    """
    threading.Timer implementation for renewing a lock

    Parameters:
            stub (object): The gRPC stub object used to communicate with the server.
            name (str): The name of the lock to renew.
            key (str): The key associated with the lock to renew.
            lock_timeout_seconds (int): The timeout in seconds for acquiring the lock.

    """

    def __init__(self, stub: object, name: str, key: str, lock_timeout_seconds: int):
        interval = max(lock_timeout_seconds - 30, 10)
        super().__init__(
            interval, renew_lock, args=(stub, name, key, lock_timeout_seconds)
        )

    def run(self):
        while not self.finished.wait(self.interval):
            self.function(*self.args, **self.kwargs)


def renew_lock(stub, name: str, key: str, lock_timeout_seconds: int):
    """
    Attempts to renew a lock.

    Args:
            stub (object): The gRPC stub object used to communicate with the server.
            name (str): The name of the lock to renew.
            key (str): The key associated with the lock to renew.
            lock_timeout_seconds (int): The timeout in seconds for acquiring the lock.

    Returns:
            object: The response object returned by the gRPC server indicating the result of the lock attempt.
    """
    rpc_msg = pb2.RenewLockRequest(
        name=name,
        key=key,
        lock_timeout_seconds=lock_timeout_seconds,
    )

    return rpc_with_retry(stub.RenewLock, rpc_msg)


def rpc_with_retry(
    rpc_func: callable,
    rpc_message: object,
    interval_seconds: int = 5,
    max_retries: int = 0,
):
    """
    Executes an RPC call with retries in case of errors.

    :param rpc_func: The RPC function to call.
    :param rpc_message: The message to send in the RPC call.
    :param interval_seconds: (Optional) The interval in seconds between retry attempts. Default is 5 seconds.
    :param max_retries: (Optional) The maximum number of retries. Default is 0 (no retries).

    :return: The response from the RPC call.
    """
    retries = 0
    while True:
        try:
            resp = rpc_func(rpc_message)
            if resp.HasField("error"):
                raise exceptions.from_rpc_error(resp.error)
            return resp
        except _InactiveRpcError as e:
            if max_retries > 0 and retries > max_retries:
                raise
            print(
                f"Encountered error {e} while trying rpc_call. "
                f"Retrying in {interval_seconds} seconds."
            )
            time.sleep(interval_seconds)
        retries += 1


@contextmanager
def lock(
    stub,
    name: str,
    wait_timeout_seconds: int = 0,
    lock_timeout_seconds: int = 0,
    raise_on_wait_timeout: bool = False,
):
    """
    A context manager that attempts to acquire a lock with the given name.

    Args:
        stub (object): The gRPC stub object used to communicate with the server.
        name (str): The name of the lock to acquire.
        wait_timeout_seconds (int, optional): How long to wait to acquire lock. Defaults to 0 - no timeout.
        lock_timeout_seconds (int, optional): The lifetime of the lock in seconds (renew to renew). Defaults to 0 - no timeout.
        raise_on_wait_timeout (bool, optional): Whether to raise a LockTimeoutError if the wait timeout is exceeded. Defaults to False.

    Yields:
        object: The response object returned by the gRPC server indicating the result of the lock attempt.

    Raises:
        RuntimeError: If the lock cannot be released after being acquired.
        LockTimeoutError: If the wait timeout is exceeded and raise_on_wait_timeout is True.

    Example:
        with lock(stub, "my_lock", wait_timeout_seconds=10, lock_timeout_seconds=600) as response:
            if response.locked:
                # Lock acquired, do something
            else:
                # Lock not acquired, handle accordingly
    """

    rpc_msg = pb2.LockRequest(
        name=name,
    )
    if wait_timeout_seconds:
        rpc_msg.wait_timeout_seconds = wait_timeout_seconds
    if lock_timeout_seconds:
        rpc_msg.lock_timeout_seconds = lock_timeout_seconds

    try:
        r = rpc_with_retry(stub.Lock, rpc_msg)
    except exceptions.LockTimeoutError:
        if raise_on_wait_timeout:
            raise

    if r.locked and lock_timeout_seconds:
        timer = RenewLockTimer(stub, name, r.key, lock_timeout_seconds)
        timer.start()
    else:
        timer = None

    yield r

    if not r.locked:
        return

    if timer:
        timer.cancel()
        timer.join()

    rpc_msg = pb2.UnlockRequest(
        name=name,
        key=r.key,
    )

    r = rpc_with_retry(stub.Unlock, rpc_msg)
    if not r.unlocked:
        raise RuntimeError(f"Failed to unlock {name}")


@contextmanager
def try_lock(stub, name: str, lock_timeout_seconds: int = 0):
    """
    A context manager that attempts to acquire a lock with the given name.

    Args:
        stub (object): The gRPC stub object used to communicate with the server.
        name (str): The name of the lock to acquire.
        lock_timeout_seconds (int, optional): The lifetime of the lock in seconds (renew to renew). Defaults to 0 - no timeout.

    Yields:
        object: The response object returned by the gRPC server indicating the result of the lock attempt.

    Raises:
        RuntimeError: If the lock cannot be released after being acquired.

    Example:
        with try_lock(stub, "my_lock", 10) as response:
            if response.locked:
                # Lock acquired, do something
            else:
                # Lock not acquired, handle accordingly
    """
    rpc_msg = pb2.TryLockRequest(
        name=name,
    )
    if lock_timeout_seconds:
        rpc_msg.lock_timeout_seconds = lock_timeout_seconds

    r = rpc_with_retry(stub.TryLock, rpc_msg)

    if r.locked and lock_timeout_seconds:
        timer = RenewLockTimer(stub, name, r.key, lock_timeout_seconds)
        timer.start()
    else:
        timer = None

    yield r

    if not r.locked:
        return

    if timer:
        timer.cancel()
        timer.join()

    rpc_msg = pb2.UnlockRequest(
        name=name,
        key=r.key,
    )

    r = rpc_with_retry(stub.Unlock, rpc_msg)
    if not r.unlocked:
        raise RuntimeError(f"Failed to unlock {name}")


def run():
    creds = grpc.ssl_channel_credentials(
        readfile(CA_CERT),
        private_key=readfile(CLIENT_KEY),
        certificate_chain=readfile(CLIENT_CERT),
    )

    with grpc.secure_channel("localhost:3144", creds) as ch:
        stub = pb2grpc.LDLMStub(ch)
        with try_lock(stub, "work-item1") as r:
            if r.locked:
                print(f"Lock for {r.name} acquired. Doing some work...")
                time.sleep(10)
            else:
                print("Failed to acquire lock")


def readfile(fl):
    with open(fl, "rb") as f:
        return f.read()


if __name__ == "__main__":
    run()
