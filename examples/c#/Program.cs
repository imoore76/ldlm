// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
protoc -I../../ --csharp_out=./protos ../../ldlm.proto
*/

using System;

using Grpc.Net.Client;
using Ldlm;
using Grpc.Core;

namespace Ldlm
{
    class Program
    {
        // Task for renewing a lock
        static async Task Renew(LDLM.LDLMClient client, RenewRequest req, CancellationToken cancellationToken)
        {
            var interval = Math.Max(req.LockTimeoutSeconds - 30, 10);
            while (true)
            {
                await Task.Delay((int)interval * 1000, cancellationToken);
                var res = await client.RenewAsync(req);
                if (res.Locked) {
                    Console.WriteLine("Renewed lock {0} with key {1}", req.Name, req.Key);
                }
            }
        }

        static async Task Main(string[] args)
        {
            var channel = GrpcChannel.ForAddress("http://localhost:3144");
            var client = new LDLM.LDLMClient(channel);

            // Lock "task-1"
            var LockRes = await client.LockAsync(
                new LockRequest
                {
                    Name = "task-1",
                    LockTimeoutSeconds = 30
                }
            );

            Console.WriteLine(LockRes);

            // Spawn renew task
            var c = new CancellationTokenSource();
            var renewTask = Program.Renew(
                client, 
                new RenewRequest { 
                    Name = "task-1",
                    Key = LockRes.Key,
                    LockTimeoutSeconds = 30 
                }, 
                c.Token
            );

            // Do some work on "task-1"
            Console.WriteLine("Doing some work that will take 60 seconds...");
            await Task.Delay(60000);
            Console.WriteLine("Done doing work...");

            // Stop lock renew task
            c.Cancel();

            // Unlock "task-1"
            var UnlockRes = await client.UnlockAsync(
                new UnlockRequest
                {
                    Name = "task-1",
                    Key = LockRes.Key
                }
            );

            Console.WriteLine(UnlockRes);

        }
    }
}
