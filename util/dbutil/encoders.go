/*
   Copyright The Soci Snapshotter Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package dbutil

import (
	"encoding/binary"
	"errors"
	"fmt"
)

func EncodeInt(i int64) ([]byte, error) {
	var (
		buf      [binary.MaxVarintLen64]byte
		iEncoded = buf[:]
	)
	iEncoded = iEncoded[:binary.PutVarint(iEncoded, i)]
	if len(iEncoded) == 0 {
		return nil, fmt.Errorf("failed encoding integer = %v", i)
	}
	return iEncoded, nil
}

func DecodeInt(data []byte) (int64, error) {
	i, n := binary.Varint(data)
	if i == 0 {
		if n == 0 {
			return 0, errors.New("not enough data")
		}
		if n < 0 {
			return 0, errors.New("data overflows int64")
		}
	}
	return i, nil
}
