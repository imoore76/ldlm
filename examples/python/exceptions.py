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

import sys

def from_rpc_error(rpc_error):
    for cls in BaseIDLMException.__subclasses__():
        if rpc_error.code == cls.RPC_CODE:
            return cls(rpc_error.code, rpc_error.message)
    raise ValueError(f"Unknown RPC error code: {rpc_error.code}")


class BaseIDLMException(Exception):
    __slots__ = ("code", "message")

    def __init__(self, code, message):
        self.code = code
        self.message = message


class IDLMException(BaseIDLMException):
    RPC_CODE = 0


class LockDoesNotExistException(BaseIDLMException):
    RPC_CODE = 1


class InvalidLockKeyException(BaseIDLMException):
    RPC_CODE = 2


class LockWaitTimeoutException(BaseIDLMException):
    RPC_CODE = 3


class NotLockedException(BaseIDLMException):
    RPC_CODE = 4


class LockDoesNotExistOrInvalidKeyException(BaseIDLMException):
    RPC_CODE = 5
