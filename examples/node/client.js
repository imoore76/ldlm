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

async function main() {
    const PROTO_PATH = __dirname + '../../../ldlm.proto';
    const grpc = require('@grpc/grpc-js');
    const protoLoader = require('@grpc/proto-loader');

    // Suggested options for similarity to existing grpc.load behavior
    const packageDefinition = protoLoader.loadSync(
        PROTO_PATH,
        {
            keepCase: true,
            longs: String,
            enums: String,
            defaults: true,
            oneofs: true
        });
    const protoDescriptor = grpc.loadPackageDefinition(packageDefinition);

    // The protoDescriptor object has the full package hierarchy
    const ldlm = protoDescriptor.ldlm;

    const client = new ldlm.LDLM(
        'localhost:3144',
        grpc.credentials.createInsecure());


    // Lock work item
    var lockResponse = await (async function() {
        return new Promise((resolve, reject) => {
            client.Lock({ name: "work-item1", lock_timeout_seconds: 30 }, function (err, res) {
                if (err) {
                    reject(err);
                } else {
                    resolve(res);
                }
            })
        })
    })()

    console.log(lockResponse);

    // Spawn lock renewer
    var lockRenew = setInterval(() => {
        client.RenewLock({
            name: "work-item1",
            key: lockResponse.key,
            lock_timeout_seconds: 30,
        }, function (err, res) {
            if (err) {
                throw new Error(err);
            } else {
                console.log("Renewed lock", res);
            }
        })
    }, 10000);

    // Do some work
    console.log("Doing work on work-item1 for 60 seconds...");
    await new Promise(resolve => setTimeout(resolve, 60000));
    console.log("Finished doing work on work-item1...");

    // Clear lock renewer
    clearInterval(lockRenew);

    // Unlock work item
    await client.Unlock({ name: "work-item1", key: lockResponse.key }, function (err, res) {
        if (err) {
            throw new Error(err);
        } else {
            console.log(res);
        }
    })

}

main().then(() => { }).catch((err) => console.error(err))