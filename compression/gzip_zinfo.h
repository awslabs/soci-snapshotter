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

/* 
  Copyright (C) 1995-2017 Jean-loup Gailly and Mark Adler
  This software is provided 'as-is', without any express or implied
  warranty.  In no event will the authors be held liable for any damages
  arising from the use of this software.
  Permission is granted to anyone to use this software for any purpose,
  including commercial applications, and to alter it and redistribute it
  freely, subject to the following restrictions:
  1. The origin of this software must not be misrepresented; you must not
     claim that you wrote the original software. If you use this software
     in a product, an acknowledgment in the product documentation would be
     appreciated but is not required.
  2. Altered source versions must be plainly marked as such, and must not be
     misrepresented as being the original software.
  3. This notice may not be removed or altered from any source distribution.
  Jean-loup Gailly        Mark Adler
  jloup@gzip.org          madler@alumni.caltech.edu
*/
/* 
  This source code is based on 
  https://github.com/madler/zlib/blob/master/examples/zran.c 
  and related code from that repository. It retains the copyright and 
  distribution restrictions of that work. It has been substantially modified 
  from the original.
*/

#ifndef GZIP_ZINFO_H
#define GZIP_ZINFO_H

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>

#include <string.h>
#include <zlib.h>

typedef unsigned char uchar;
typedef int64_t offset_t;

#define ZINFO_VERSION_ONE 1
#define ZINFO_VERSION_TWO 2

#define ZINFO_VERSION_CUR ZINFO_VERSION_TWO

/* Since gzip is compressed with 32 KiB window size, WINDOW_SIZE is fixed */
#define WINSIZE 32768U

/*
    -  8 bytes, compressed offset
    -  8 bytes, uncompressed offset
    -  1 byte, bits
    -  32768 bytes, window
*/
#define PACKED_CHECKPOINT_SIZE (8 + 8 + 1 + WINSIZE)

/*
    -  4 bytes, number of checkpoints
    -  8 bytes, span size
*/
#define BLOB_HEADER_SIZE (4 + 8)


enum {
    GZIP_ZINFO_OK = 0,
    GZIP_ZINFO_FILE_NOT_FOUND = -80,
    GZIP_ZINFO_INDEX_NULL = -81,
    GZIP_ZINFO_CANNOT_ALLOC = -82,
};

struct gzip_checkpoint {
    offset_t out;          /* corresponding offset in uncompressed data */
    offset_t in;           /* offset in input file of first full byte */
    uint8_t bits;           /* number of bits (1-7) from byte at in - 1, or 0 */
    unsigned char window[WINSIZE];  /* preceding 32K of uncompressed data */    
};

struct gzip_zinfo {
    int32_t version;
    int32_t have;           /* number of list entries filled in */
    int32_t size;           /* number of list entries allocated */
    struct gzip_checkpoint *list; /* allocated list */
    offset_t span_size;
};

// zinfo - metadata starts.
// Get index number of gzip zinfo within which the uncompressed offset is present
int         pt_index_from_ucmp_offset(struct gzip_zinfo *index, offset_t off);
offset_t    get_ucomp_off(struct gzip_zinfo *index, int checkpoint);
offset_t    get_comp_off(struct gzip_zinfo *index, int checkpoint);
unsigned    get_blob_size(struct gzip_zinfo *index);
int32_t     get_max_span_id(struct gzip_zinfo *index);
int         has_bits(struct gzip_zinfo *index, int checkpoint);
// zinfo - metadata ends.

// zinfo - generation/extraction starts.
int generate_zinfo_from_file(const char* filepath, offset_t span, struct gzip_zinfo** index);
int extract_data_from_file(const char* file, struct gzip_zinfo* index, offset_t offset, void* buf, int len);
int extract_data_from_buffer(void* d, offset_t datalen, struct gzip_zinfo* index, offset_t offset, void* buffer, offset_t len, int first_checkpoint);
// zinfo - generation/extraction ends.

// zinfo -  zinfo <-> blob conversion starts.
/* Converts zinfo to blob
   Returns the size of the buffer on success
   This function assumes that the buffer is large enough already
   to hold the entire zinfo
*/
int     zinfo_to_blob(struct gzip_zinfo* index, void* buf);
struct  gzip_zinfo* blob_to_zinfo(void* buf, offset_t len);
// zinfo -  zinfo <-> blob conversion ends.

#endif // GZIP_ZINFO_H
