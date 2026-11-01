/* Copyright 2026 ALTESSA SOLUTIONS INC.
 * Use of this source code is governed by an MIT-style license that can be
 * found in the LICENSE file. */
#ifndef RMD_INTERNAL_H
#define RMD_INTERNAL_H
#include <assert.h>
#include <limits.h>
#include <rmdoc/rmdoc.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct rmd_name_node {
    struct rmd_name_node *left, *right;
    uint32_t *key;
    size_t length;
    int height;
} rmd_name_node;
typedef struct {
    size_t payload, start, end;
} rmd_location;
struct rmd_reader {
    rmd_bytes data;
    rmd_limits limits;
    rmd_entry *entries;
    rmd_location *locations;
    size_t count;
    bool mapped;
};
void *rmd_alloc(size_t n);
void *rmd_calloc(size_t n, size_t size);
void *rmd_realloc(void *p, size_t n);
#ifdef RMDOC_TESTING
void rmd_test_fail_after(size_t count);
#endif
rmd_status rmd_fail(rmd_error *e, rmd_status code, rmd_str name, uint64_t actual, uint64_t limit,
                    const char *message);
#define RMD_FAIL(e, code, message) rmd_fail(e, code, (rmd_str){0}, 0, 0, message)
size_t rmd_utf8_next(rmd_str s, size_t at, uint32_t *cp);
bool rmd_utf8_valid(rmd_str s);
rmd_status rmd_names_insert(rmd_name_node **tree, rmd_str name);
void rmd_names_free(rmd_name_node *tree);
rmd_status rmd_names_check(const rmd_name_node *tree, rmd_str name);
rmd_status rmd_inflate(rmd_bytes data, uint32_t expected, rmd_buffer *out);
rmd_status rmd_deflate(rmd_bytes data, rmd_buffer *out);
uint32_t rmd_crc32(uint32_t crc, rmd_bytes data);
rmd_status rmd_contents(const rmd_reader *r, size_t index, rmd_buffer *out, rmd_error *e);
FILE *rmd_fopen(const char *path, bool write);
int rmd_seek(FILE *file, uint64_t position);
void rmd_unmap(rmd_bytes data);
static inline uint16_t rmd_u16(const uint8_t *p) {
    return (uint16_t)((uint16_t)p[0] | (uint16_t)p[1] << 8);
}
static inline uint32_t rmd_u32(const uint8_t *p) {
    return (uint32_t)rmd_u16(p) | (uint32_t)rmd_u16(p + 2) << 16;
}
static inline void rmd_put16(uint8_t *p, uint16_t x) {
    p[0] = (uint8_t)x;
    p[1] = (uint8_t)(x >> 8);
}
static inline void rmd_put32(uint8_t *p, uint32_t x) {
    rmd_put16(p, (uint16_t)x);
    rmd_put16(p + 2, (uint16_t)(x >> 16));
}
#endif
