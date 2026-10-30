#ifndef HDMA_NATIVE_H
#define HDMA_NATIVE_H

#include <stdint.h>

/* Shared metadata contract: bits is reported effective precision; explicit
 * lossy processing uses 32 to request full-width PCM without zero-padding
 * assertions. layout names speaker positions; text must be NUL-terminated.
 * index identifies the global container stream. Operations revalidate this
 * metadata snapshot, not a fingerprint of the file's contents. */
typedef struct HDMAInfo {
    int index;
    int rate;
    int bits;
    int channels;
    char codec[64];
    char profile[64];
    char layout[128];
    char language[64];
    int64_t delay_ms;
} HDMAInfo;

const char *hdma_version(void);
int hdma_check(void);
void hdma_init_logging(void);
uintptr_t hdma_log_enter(uintptr_t handle);
void hdma_log_leave(uintptr_t previous);
const char *hdma_error_kind(int code);
int hdma_probe(const char *path, uintptr_t handle, HDMAInfo *info, char *error);
typedef struct HDMATrack {
    HDMAInfo info;
    char title[512];
    int is_default;
    int64_t duration_us;
} HDMATrack;
/* Enumeration owns a C allocation until hdma_tracks_free. Stream indices are
 * container indices, not ordinal positions in the audio-only result list. */
int hdma_probe_tracks(const char *path, uintptr_t handle, HDMATrack **tracks,
                      int *count, char *error);
void hdma_tracks_free(HDMATrack *tracks);
/* index == -1 preserves the one-stream input contract; nonnegative indices
 * select that exact audio stream, regardless of default-track disposition. */
int hdma_probe_stream(const char *path, int index, uintptr_t handle,
                      HDMAInfo *info, char *error);
/* output == NULL selects PCM callbacks; otherwise encode to that private path.
 * The caller supplies a 512-byte error buffer and a live Go callback handle
 * for each synchronous call. A negative return is an FFmpeg error code. */
int hdma_process(const char *path, const char *output, const HDMAInfo *expected,
                 uintptr_t handle, char *error);

/* Shared AC-3/E-AC-3 lifecycle, retaining the original EAC3 symbol names.
 * write accepts interleaved, left-aligned little-endian int32 PCM at 48 kHz.
 * bitrate is total bits/second; keep handle live until close, even on failure.
 * finish returns source and padded decoded samples per channel; close is still
 * required after finish succeeds. Neither operation publishes the final path. */
typedef struct HDMAEAC3 HDMAEAC3;
int hdma_eac3_open(const char *path, int channels, int bitrate,
                   uintptr_t handle, HDMAEAC3 **result, char *error);
/* AC-3 uses the same PCM/write/finish lifecycle, fixed at 640 kb/s, 5.1(side). */
int hdma_ac3_open(const char *path, int channels, uintptr_t handle,
                  HDMAEAC3 **result, char *error);
int hdma_eac3_write(HDMAEAC3 *op, const unsigned char *pcm, int size, char *error);
int hdma_eac3_finish(HDMAEAC3 *op, uint64_t *samples, uint64_t *decoded, char *error);
void hdma_eac3_close(HDMAEAC3 *op);

#endif
