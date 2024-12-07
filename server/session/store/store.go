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
This file contains the store struct, its methods, and some helper functions related to
serialization. store is responsible for reading and writing the server's session locks to a file
so that locks can persist across ldlm server restarts.
*/
package store

import (
	"fmt"
	"io"
	"os"

	"github.com/deneonet/benc"
	bstd "github.com/deneonet/benc/std"
	cl "github.com/imoore76/go-ldlm/server/clientlock"
)

// store provides Read() and Write() functions to read and write
// the server's session locks map. The state file is never closed - only
// truncated, rewritten, and sync()ed
type store struct {
	fh *os.File
}

// New returns a new store instance
func New(stateFile string) (*store, error) {
	var fh *os.File
	if stateFile != "" {
		var err error
		fh, err = os.OpenFile(stateFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return nil, err
		}
	}

	return &store{
		fh: fh,
	}, nil

}

// Write writes the server's session locks map to the store's state file
func (l *store) Write(sessionLocks map[string][]cl.Lock) error {

	// No state file configured. Nothing to do
	if l.fh == nil {
		return nil
	}

	d := marshalLocks(sessionLocks)
	l.fh.Truncate(0)
	l.fh.Seek(0, io.SeekStart)
	if _, err := l.fh.Write(d); err != nil {
		panic(err)
	}
	l.fh.Sync()
	return nil
}

// Read reads serialized client lock map from store's state file
// directly into the server's session locks map
func (l *store) Read() (map[string][]cl.Lock, error) {

	// No state file configured. Nothing to do
	if l.fh == nil {
		return nil, nil
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
		return nil, nil
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

	if m, err := unmarshalLocks(data); err != nil {
		return nil, fmt.Errorf("error unmarshaling state data: %w", err)
	} else {
		return m, nil
	}
}

// Close closes the store's state file
func (l *store) Close() {
	if l.fh != nil {
		l.fh.Close()
	}
}

func lockSize(clk cl.Lock) (s int) {
	// Calculate the size of `clk` (cl.Lock)
	s = bstd.SizeString(clk.Name())
	s2 := bstd.SizeString(clk.Key())
	s += s2 + bstd.SizeInt32()
	return
}

// marshalLock marshals a single client lock struct
func marshalLock(n int, b []byte, l cl.Lock) int {
	// Serialize the struct into a byte slice
	n = bstd.MarshalString(n, b, l.Name())
	n = bstd.MarshalString(n, b, l.Key())
	return bstd.MarshalInt32(n, b, l.Size())
}

// unmarshalLock unmarshals a single client lock struct
func unmarshalLock(n int, b []byte) (int, cl.Lock, error) {
	// error cl.Lock return
	dft := cl.New("", "", 0)

	var name, key string
	// Deserialize the byte slice into the struct
	n, name, err := bstd.UnmarshalString(n, b)
	if err != nil {
		return 0, dft, err
	}

	n, key, err = bstd.UnmarshalString(n, b)
	if err != nil {
		return 0, dft, err
	}

	n, size, err := bstd.UnmarshalInt32(n, b)
	if err != nil {
		return 0, dft, err
	}

	return n, cl.New(name, key, size), nil
}

// Marshal client lock map to byte slice for writing
func marshalLocks(m map[string][]cl.Lock) []byte {
	sz := bstd.SizeMap(m, bstd.SizeString, func(v []cl.Lock) int {
		return bstd.SizeSlice(v, lockSize)
	})

	b := make([]byte, sz)
	bstd.MarshalMap(0, b, m, bstd.MarshalString, func(n int, b []byte, v []cl.Lock) int {
		return bstd.MarshalSlice(n, b, v, marshalLock)
	})

	return b
}

// Unmarshal byte slice to client lock map
func unmarshalLocks(b []byte) (map[string][]cl.Lock, error) {
	// See marshalLocks() for file format notes
	n, m, err := bstd.UnmarshalMap[string, []cl.Lock](0, b, bstd.UnmarshalString, func(n int, b []byte) (int, []cl.Lock, error) {
		n, s, err := bstd.UnmarshalSlice[cl.Lock](n, b, unmarshalLock)
		return n, s, err
	})
	if err != nil {
		return nil, err
	}

	return m, benc.VerifyMarshal(n, b)
}
