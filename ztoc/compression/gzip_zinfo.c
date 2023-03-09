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

#include "gzip_zinfo.h"

#include <endian.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define CHUNK (1 << 14) // file input buffer size


// zinfo - internal helpers start.

/* Convert integer types to little endian and vice versa.
   This is needed to keep zinfo consistent across multiple architectures,
   ensuring that all integer fields will be stored in little endian.
*/
inline offset_t encode_offset(offset_t source) {
    return htole64(source);
}

inline offset_t decode_offset(offset_t source) {
    return le64toh(source);
}

inline int32_t encode_int32(int32_t source) {
    return htole32(source);
}

inline int32_t decode_int32(int32_t source) {
    return le32toh(source);
}

static int min(int lhs, int rhs) { return lhs < rhs ? lhs : rhs; }

int init_flate(z_stream *strm, int windowBits) {
    int ret;
    strm->zalloc = Z_NULL;
    strm->zfree = Z_NULL;
    strm->opaque = Z_NULL;
    strm->avail_in = 0;
    strm->next_in = Z_NULL;
    ret = inflateInit2(strm, windowBits);
    return ret;
}

static uint8_t get_bits(struct gzip_zinfo *index, int checkpoint) {
    return index->list[checkpoint].bits;
}

// zinfo - internal helpers end.

// zinfo - metadata starts.
int pt_index_from_ucmp_offset(struct gzip_zinfo* index, offset_t off) {
    if (index == NULL)
        return -1;

    int res = 0;
    struct gzip_checkpoint* here = index->list;
    int ret = decode_int32(index->have);
    while (--ret && decode_offset(here[1].out) <= off) {
        here++;
        res++;
    }
    return res;
}

offset_t get_ucomp_off(struct gzip_zinfo *index, int checkpoint) {
    return decode_offset(index->list[checkpoint].out);
}

offset_t get_comp_off(struct gzip_zinfo *index, int checkpoint) {
    return decode_offset(index->list[checkpoint].in);
}

unsigned get_blob_size(struct gzip_zinfo *index) {
    if (index == NULL)
        return 0;

    unsigned size = decode_int32(index->size);
    if (decode_int32(index->version) == ZINFO_VERSION_ONE)
        size--;

    /*
        The buffer will be tightly packed. The layout of the buffer is:
        -   Some fixed size based on which version
        -   PACKED_CHECKPOINT_SIZE for each span.
            If we have a v1 gzip_zinfo, we skip the first checkpoint
            this is a bug, but it keeps backwards compatibility
    */
    return PACKED_CHECKPOINT_SIZE * size + BLOB_HEADER_SIZE;
}

int32_t get_max_span_id(struct gzip_zinfo *index) {
    if (index == NULL)
        return 0;
    return decode_int32(index->have) - 1;
}

int has_bits(struct gzip_zinfo *index, int checkpoint) {
    if (checkpoint >= decode_int32(index->have))
        return 0;
    return index->list[checkpoint].bits != 0;
}

// zinfo - metadata ends.

void free_zinfo(struct gzip_zinfo *index) {
    if (index != NULL) {
        free(index->list);
        free(index);
    }
}

// zinfo - generation/extraction starts.

/* Add an entry to the access point list.  If out of memory, deallocate the
   existing list and return NULL. */
static struct gzip_zinfo *add_checkpoint(struct gzip_zinfo *index, uint8_t bits,
    offset_t in, offset_t out, unsigned left, unsigned char *window) {
    struct gzip_checkpoint *next;

    if (index == NULL) {
        /* if list is empty, create it (start with eight points) */
        index = malloc(sizeof(struct gzip_zinfo));
        if (index == NULL) return NULL;
        index->list = malloc(sizeof(struct gzip_checkpoint) << 3);
        if (index->list == NULL) {
            free(index);
            return NULL;
        }
        index->size = 8;
        index->have = 0;
    } else if (index->have == index->size) {
        /* if list is full, make it bigger */
        index->size <<= 1;
        next = realloc(index->list, sizeof(struct gzip_checkpoint) * index->size);
        if (next == NULL) {
            free_zinfo(index);
            return NULL;
        }
        index->list = next;
    }

    /* fill in entry and increment how many we have */
    next = index->list + index->have;
    next->bits = bits;
    next->in = encode_offset(in);
    next->out = encode_offset(out);
    if (left)
        memcpy(next->window, window + WINSIZE - left, left);
    if (left < WINSIZE)
        memcpy(next->window + left, window, WINSIZE - left);
    index->have++;

    /* return list, possibly reallocated */
    return index;
}

/* Pretty much the same as from zran.c */
int generate_zinfo_from_fp(FILE* in, offset_t span, struct gzip_zinfo** idx) {
    int ret;
    offset_t totin, totout;        /* our own total counters to avoid 4GB limit */
    offset_t last;                 /* totout value of last access point */
    struct gzip_zinfo *index;       /* access points being generated */
    z_stream strm;
    unsigned char input[CHUNK], window[WINSIZE];
    memset(window, 0, WINSIZE);

    /* initialize inflate */
    ret = init_flate(&strm, 47); /* automatic zlib or gzip decoding */
    if (ret != Z_OK)
        return ret;

    /* inflate the input, maintain a sliding window, and build an index -- this
       also validates the integrity of the compressed data using the check
       information at the end of the gzip or zlib stream */
    totin = totout = last = 0;
    index = NULL;               /* will be allocated by first add_checkpoint() */
    strm.avail_out = 0;
    do {
        /* get some compressed data from input file */
        memset(input, 0, CHUNK);
        strm.avail_in = fread(input, 1, CHUNK, in);
        if (ferror(in)) {
            ret = Z_ERRNO;
            goto build_index_error;
        }
        if (strm.avail_in == 0) {
            ret = Z_DATA_ERROR;
            goto build_index_error;
        }
        strm.next_in = input;

        /* process all of that, or until end of stream */
        do {
            /* reset sliding window if necessary */
            if (strm.avail_out == 0) {
                strm.avail_out = WINSIZE;
                strm.next_out = window;
            }

            /* inflate until out of input, output, or at end of block --
               update the total input and output counters */
            totin += strm.avail_in;
            totout += strm.avail_out;
            ret = inflate(&strm, Z_BLOCK);      /* return at end of block */
            totin -= strm.avail_in;
            totout -= strm.avail_out;
            if (ret == Z_NEED_DICT)
                ret = Z_DATA_ERROR;
            if (ret == Z_MEM_ERROR || ret == Z_DATA_ERROR)
                goto build_index_error;
            if (ret == Z_STREAM_END)
                break;

            /* if at end of block, consider adding an index entry (note that if
               data_type indicates an end-of-block, then all of the
               uncompressed data from that block has been delivered, and none
               of the compressed data after that block has been consumed,
               except for up to seven bits) -- the totout == 0 provides an
               entry point after the zlib or gzip header, and assures that the
               index always has at least one access point; we avoid creating an
               access point after the last block by checking bit 6 of data_type
             */
            if ((strm.data_type & 128) && !(strm.data_type & 64) &&
                (totout == 0 || totout - last > span)) {
                index = add_checkpoint(index, (uint8_t)(strm.data_type & 7), totin,
                                 totout, strm.avail_out, window);
                if (index == NULL) {
                    ret = Z_MEM_ERROR;
                    goto build_index_error;
                }
                last = totout;
            }
        } while (strm.avail_in != 0);
    } while (ret != Z_STREAM_END);

    /* clean up and return index (release unused entries in list) */
    (void)inflateEnd(&strm);
    index->list = realloc(index->list, sizeof(struct gzip_checkpoint) * index->have);
    index->size = index->have;
    index->have = encode_int32(index->have);
    int32_t sz = index->size;
    index->size = encode_int32(index->size);
    index->span_size = encode_offset(span);
    index->version = encode_int32(ZINFO_VERSION_CUR);
    *idx = index;
    return sz;

    /* return error */
  build_index_error:
    (void)inflateEnd(&strm);
    free_zinfo(index);
    return ret;
}

int generate_zinfo_from_file(const char *filepath, offset_t span, struct gzip_zinfo **index) {
    FILE *fp = fopen(filepath, "rb");
    if (fp == NULL)
        return GZIP_ZINFO_FILE_NOT_FOUND;
    int ret = generate_zinfo_from_fp(fp, span, index);
    fclose(fp);
    return ret;
}

int extract_data_from_fp(FILE *in, struct gzip_zinfo *index, offset_t offset, void *buffer, int len) {
    int ret, skip;
    z_stream strm;
    struct gzip_checkpoint *here;
    unsigned char input[CHUNK], discard[WINSIZE];
    uchar* buf = buffer; 

    /* proceed only if something reasonable to do */
    if (len < 0)
        return 0;

    /* find where in stream to start */
    here = index->list;
    ret = decode_int32(index->have);
    while (--ret && decode_offset(here[1].out) <= offset)
        here++;

    /* initialize inflate */
    ret = init_flate(&strm, -15); /* raw inflate */
    if (ret != Z_OK)
        return ret;

    ret = fseeko(in, decode_offset(here->in) - (here->bits ? 1 : 0), SEEK_SET);
    if (ret == -1)
        goto extract_ret;
    if (here->bits) {
        ret = getc(in);
        if (ret == -1) {
            ret = ferror(in) ? Z_ERRNO : Z_DATA_ERROR;
            goto extract_ret;
        }
        (void)inflatePrime(&strm, here->bits, ret >> (8 - here->bits));
    }
    (void)inflateSetDictionary(&strm, here->window, WINSIZE);
    /* skip uncompressed bytes until offset reached, then satisfy request */
    offset -= decode_offset(here->out);
    strm.avail_in = 0;
    skip = 1;                               /* while skipping to offset */
    do {
        /* define where to put uncompressed data, and how much */
        if (offset == 0 && skip) {          /* at offset now */
            strm.avail_out = len;
            strm.next_out = buf;
            skip = 0;                       /* only do this once */
        }
        if (offset > WINSIZE) {             /* skip WINSIZE bytes */
            strm.avail_out = WINSIZE;
            strm.next_out = discard;
            offset -= WINSIZE;
        } else if (offset != 0) {             /* last skip */
            strm.avail_out = (unsigned)offset;
            strm.next_out = discard;
            offset = 0;
        }
        /* uncompress until avail_out filled, or end of stream */
        do {
            if (strm.avail_in == 0) {
                strm.avail_in = fread(input, 1, CHUNK, in);
                if (ferror(in)) {
                    ret = Z_ERRNO;
                    goto extract_ret;
                }
                if (strm.avail_in == 0) {
                    ret = Z_DATA_ERROR;
                    goto extract_ret;
                }
                strm.next_in = input;
            }
            ret = inflate(&strm, Z_NO_FLUSH);       /* normal inflate */
            if (ret == Z_NEED_DICT)
                ret = Z_DATA_ERROR;
            if (ret == Z_MEM_ERROR || ret == Z_DATA_ERROR)
                goto extract_ret;
            if (ret == Z_STREAM_END)
                break;
        } while (strm.avail_out != 0);

        /* if reach end of stream, then don't keep trying to get more */
        if (ret == Z_STREAM_END)
            break;
        /* do until offset reached and requested data read, or stream ends */
    } while (skip);

    /* compute number of uncompressed bytes read after offset */
    ret = skip ? 0 : len - strm.avail_out;

    /* clean up and return bytes read or error */
  extract_ret:
    (void)inflateEnd(&strm);
    return ret;
}

int extract_data_from_file(const char* file, struct gzip_zinfo* index, offset_t offset, void* buf, int len) {
    FILE* fp = fopen(file, "rb");
    if (fp == NULL) 
        return GZIP_ZINFO_FILE_NOT_FOUND;

    int ret = extract_data_from_fp(fp, index, offset, buf, len);
    fclose(fp);
    return ret;
}

// This is the same as extract_data_fp, but instead of a file, it decompresses
// data from a buffer which contains the exact data to decompress
int extract_data_from_buffer(void *d, offset_t datalen,
                             struct gzip_zinfo *index, offset_t offset,
                             void *buffer, offset_t len, int first_checkpoint) {
    int ret, skip;
    z_stream strm;
    unsigned char input[CHUNK], discard[WINSIZE];
    uchar *buf = buffer;
    uchar *data = d;
    /* proceed only if something reasonable to do */
    if (len < 0)
        return 0;

    uint8_t bits = get_bits(index, first_checkpoint);

    /* initialize inflate */
    ret = init_flate(&strm, -15); /* raw inflate */
    if (ret != Z_OK)
        return ret;

    if (bits) {
        int ret = data[0];
        inflatePrime(&strm, bits, ret >> (8 - bits));
        data++;
    }
    (void)inflateSetDictionary(&strm, index->list[first_checkpoint].window,
                               WINSIZE);
    offset -= decode_offset(index->list[first_checkpoint].out);
    strm.avail_in = 0;
    skip = 1; /* while skipping to offset */
    int remaining = datalen;
    do {
        /* define where to put uncompressed data, and how much */
        if (offset == 0 && skip) { /* at offset now */
            strm.avail_out = len;
            strm.next_out = buf;
            skip = 0; /* only do this once */
        }
        if (offset > WINSIZE) { /* skip WINSIZE bytes */
            strm.avail_out = WINSIZE;
            strm.next_out = discard;
            offset -= WINSIZE;
        } else if (offset != 0) { /* last skip */
            strm.avail_out = (unsigned)offset;
            strm.next_out = discard;
            offset = 0;
        }
        /* uncompress until avail_out filled, or end of stream */
        do {
            if (strm.avail_in == 0) {
                int read = min(remaining, CHUNK);
                remaining -= read;
                memcpy(input, data, read);
                data += read;
                strm.avail_in = read;
                strm.next_in = input;
            }
            ret = inflate(&strm, Z_NO_FLUSH); /* normal inflate */
            if (ret == Z_NEED_DICT)
                ret = Z_DATA_ERROR;
            if (ret == Z_MEM_ERROR || ret == Z_DATA_ERROR)
                goto extract_ret;
            if (ret == Z_STREAM_END)
                break;
        } while (strm.avail_out != 0);

        /* if reach end of stream, then don't keep trying to get more */
        if (ret == Z_STREAM_END)
            break;

        /* do until offset reached and requested data read, or stream ends */
    } while (skip);

    /* compute number of uncompressed bytes read after offset */
    ret = skip ? 0 : len - strm.avail_out;

    /* clean up and return bytes read or error */
extract_ret:
    (void)inflateEnd(&strm);
    return ret;
}

// zinfo - generation/extraction ends.

// zinfo -  zinfo <-> blob conversion starts.
int zinfo_to_blob(struct gzip_zinfo* index, void* buf) {
    if (index == NULL)
        return GZIP_ZINFO_INDEX_NULL;

    // TODO: Since this will be serialized to file, we need to be mindful of endianness. Right now, we are just ignoring it
    // Or maybe not, since Golang might take care of it 
    if (buf == NULL)
       return 0;

    uchar* cur = buf;
    int32_t first_checkpoint_index;
    memcpy(cur, &index->have, 4);
    cur += 4;
    memcpy(cur, &index->span_size, 8);
    cur += 8;
    first_checkpoint_index = 0;
    // in v1, we skipped the 0th checkpoint becasue we assumed it was fixed size.
    // in v2, we encode the 0th block because it's not a fixed size if gzip headers are used.
    // for backwards compatibility we want to reserialize v1 zinfo to exactly the same bytes
    // even though there is technically a bug.
    if (decode_int32(index->version) == ZINFO_VERSION_ONE)
        first_checkpoint_index = 1;

    for(int i = first_checkpoint_index; i < decode_int32(index->have); i++) {
        struct gzip_checkpoint* pt = &index->list[i];
        memcpy(cur, &pt->in, 8);
        cur += 8;
        memcpy(cur, &pt->out, 8);
        cur += 8;
        memcpy(cur, &pt->bits, 1);
        cur += 1;
        memcpy(cur, &pt->window, WINSIZE);
        cur += WINSIZE;
    }

    return get_blob_size(index);
}

struct gzip_zinfo* blob_to_zinfo(void* buf, offset_t len) {
    if (buf == NULL)
        return NULL;

    if (len < BLOB_HEADER_SIZE)
        return NULL;

    struct gzip_zinfo* index = malloc(sizeof(struct gzip_zinfo));
    if (index == NULL)
        return NULL;

    int32_t size, first_checkpoint_index, version;
    offset_t claimed_size, span_size;

    uchar* cur = buf;
    memcpy(&size, cur, 4);
    cur += 4;
    memcpy(&span_size, cur, 8);
    cur += 8;

    claimed_size = PACKED_CHECKPOINT_SIZE * decode_int32(size) + BLOB_HEADER_SIZE;
    if (claimed_size == len) {
        // If we have exactly size checkpoints, then we have a current blob
        version = ZINFO_VERSION_CUR;
    } else if (claimed_size - PACKED_CHECKPOINT_SIZE == len) {
        // If we only have size - 1 checkpoints, then we have a v1 blob
        version = ZINFO_VERSION_ONE;
    } else {
        // size is invalid. don't attempt to deserialize any more data.
        return NULL;
    }

    index->list = malloc(sizeof(struct gzip_checkpoint) * decode_int32(size));
    if (index->list == NULL) {
        free_zinfo(index);
        return NULL;
    }

    first_checkpoint_index = 0;
    index->version = encode_int32(version);

    if (version == ZINFO_VERSION_ONE) {
        first_checkpoint_index = 1; 
        struct gzip_checkpoint* pt0 = &index->list[0];
        pt0->bits = 0; 
        pt0->in = encode_offset(10);
        pt0->out = 0;
        memset(pt0->window, 0, WINSIZE);
    }

    for(int32_t i = first_checkpoint_index; i < decode_int32(size); i++) {
        struct gzip_checkpoint* pt = &index->list[i];
        memcpy(&pt->in, cur, 8);
        cur += 8;
        memcpy(&pt->out, cur, 8);
        cur += 8;
        memcpy(&pt->bits, cur, 1);
        cur += 1;
        memcpy(&pt->window, cur, WINSIZE);
        cur += WINSIZE;
    }

    index->have = size;
    index->size = size;
    index->span_size = span_size;

    return index;
}

// zinfo -  zinfo <-> blob conversion ends.
