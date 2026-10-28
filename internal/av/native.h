#ifndef HDMA_NATIVE_H
#define HDMA_NATIVE_H

#include <stdint.h>

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
int hdma_process(const char *path, const char *output, const HDMAInfo *expected,
                 uintptr_t handle, char *error);

#endif
