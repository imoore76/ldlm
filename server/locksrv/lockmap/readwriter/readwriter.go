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
This file contains the readWriter struct, its methods, and some helper
functions related to serialization. readWriter is responsible for reading
and writing the server's clientLocks to a file so that locks can persist
across ldlm server restarts.
*/
package readwriter

import (
	"fmt"
	"io"
	"os"

	bstd "github.com/deneonet/benc"

	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
)

type Config struct {
	StateFile string `desc:"File in which to store lock state" default:"" short:"s"`
}

// readWriter provides Read() and Write() functions to read and write
// the server's clientLocks map. The state file is never closed - only
// truncated, rewritten, and sync()ed
type readWriter struct {
	fh *os.File
}

// New returns a new readWriter instance
func New(c *Config) (*readWriter, error) {
	var fh *os.File
	if c.StateFile != "" {
		var err error
		fh, err = os.OpenFile(c.StateFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return nil, err
		}
	}

	return &readWriter{
		fh: fh,
	}, nil

}

// Write writes the server's clientLocks map to the readWriter's state file
func (l *readWriter) Write(clientLocks map[string][]cl.ClientLock) error {

	// No state file configured. Nothing to do
	if l.fh == nil {
		return nil
	}

	if d, err := marshalClientLocks(clientLocks); err != nil {
		panic("marshalClientLocks() error: " + err.Error())
	} else {
		l.fh.Truncate(0)
		l.fh.Seek(0, io.SeekStart)
		if _, err = l.fh.Write(d); err != nil {
			panic(err)
		}
		l.fh.Sync()
	}
	return nil
}

// Read reads serialized client lock map from readWriter's state file
// directly into the server's clientLocks map
func (l *readWriter) Read() (map[string][]cl.ClientLock, error) {

	// No state file configured. Nothing to do
	if l.fh == nil {
		return map[string][]cl.ClientLock{}, nil
	}

	// SeekEnd to get size
	size, err := l.fh.Seek(0, io.SeekEnd)
	if err != nil {
		panic("SeekEnd() error: " + err.Error())
	}

	// SeekStart to read from beginning of file
	l.fh.Seek(0, io.SeekStart)

	// Empty state file
	if size == 0 {
		return map[string][]cl.ClientLock{}, nil
	}

	// < os.ReadFile source > - see os.Readfile() comments
	// "some files don't work right if read in small pieces"
	if size < 512 {
		size = 512
	}
	data := make([]byte, 0, size+1)
	for {
		n, err := l.fh.Read(data[len(data):cap(data)])
		// Trim byte slice to number of bytes read + 1
		data = data[:len(data)+n]
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
	}
	// </ os.ReadFile source >

	if m, err := unmarshalClientLocks(data); err != nil {
		return nil, fmt.Errorf("error unmarshaling state data: %w", err)
	} else {
		return m, nil
	}
}

// Close closes the readWriter's state file
func (l *readWriter) Close() {
	if l.fh != nil {
		l.fh.Close()
	}
}

// Marshal client lock map to byte slice for writing
func marshalClientLocks(m map[string][]cl.ClientLock) ([]byte, error) {
	// It seems benc doesn't handle a map of slices natively, so the format had
	// to be created. It is:
	// The first Uint size of bytes is the length of the map.
	// 		* For each map element
	// 			* The string key
	// 			* Its clientLocks slice
	sz := bstd.SizeUInt()
	for k, v := range m {
		sz += bstd.SizeString(k) + bstd.SizeSlice(v, func(clk cl.ClientLock) int {
			return bstd.SizeString(clk.Name()) + bstd.SizeString(clk.Key())
		})
	}
	n, buf := bstd.Marshal(sz)
	n = bstd.MarshalUInt(n, buf, uint(len(m)))
	for k, v := range m {
		n = bstd.MarshalString(n, buf, k)
		n = bstd.MarshalSlice(n, buf, v, marshalClientLock)
	}
	return buf, bstd.VerifyMarshal(n, buf)
}

// Unmarshal byte slice to client lock map
func unmarshalClientLocks(b []byte) (map[string][]cl.ClientLock, error) {
	// See marshalClientLocks() for file format notes
	m := make(map[string][]cl.ClientLock)
	if len(b) == 0 {
		return m, nil
	}
	var n int = 0
	var maplength uint

	n, maplength, err := bstd.UnmarshalUInt(n, b)
	if err != nil {
		panic(err)
	}

	var k string
	var l []cl.ClientLock
	for range maplength {
		// Key
		n, k, err = bstd.UnmarshalString(n, b)
		if err != nil {
			panic(err)
		}
		// Slice of clientLocks
		n, l, err = bstd.UnmarshalSlice(n, b, unmarshalClientLock)
		if err != nil {
			panic(err)
		}
		m[k] = l
	}

	return m, nil
}

// marshalClientLock marshals a single clientLock struct
func marshalClientLock(n int, b []byte, l cl.ClientLock) int {
	// Calculate the size of the struct
	sz := bstd.SizeString(l.Name()) + bstd.SizeString(l.Key())

	// Serialize the struct into a byte slice
	nn, buf := bstd.Marshal(sz)
	nn = bstd.MarshalString(nn, buf, l.Name())
	bstd.MarshalString(nn, buf, l.Key())

	copy(b[n:n+sz], buf)

	// Return new offset in the byte slice
	return n + sz
}

// unmarshalClientLock unmarshals a single clientLock struct
func unmarshalClientLock(n int, b []byte) (int, cl.ClientLock, error) {
	clk := *cl.New("", "")

	var name, key string
	// Deserialize the byte slice into the struct
	n, str, err := bstd.UnmarshalString(n, b)
	if err != nil {
		return 0, clk, err
	}
	name = str

	n, str, err = bstd.UnmarshalString(n, b)
	if err != nil {
		return 0, clk, err
	}
	key = str

	return n, *cl.New(name, key), nil
}
