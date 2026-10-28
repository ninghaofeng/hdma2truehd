#ifndef HDMA_NATIVE_H
#define HDMA_NATIVE_H

#include <stdint.h>

/* Shared metadata contract: bits is effective precision, layout names speaker
 * positions, and all text fields must be NUL-terminated. Native operations
 * revalidate this snapshot instead of trusting a stale probe result. */
typedef struct HDMAInfo {
    int index;
    int rate;
    int bits;
    int channels;
    char codec[64];
    char profile[64];
    char layout[128];
} HDMAInfo;

const char *hdma_version(void);
int hdma_check(void);
int hdma_probe(const char *path, uintptr_t handle, HDMAInfo *info, char *error);
/* output == NULL selects PCM callbacks; otherwise encode to that private path.
 * The caller supplies a 512-byte error buffer and a live Go callback handle
 * for each synchronous call. A negative return is an FFmpeg error code. */
int hdma_process(const char *path, const char *output, const HDMAInfo *expected,
                 uintptr_t handle, char *error);

#endif
