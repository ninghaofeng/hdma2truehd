#include "native.h"
#include <errno.h>
#include <stdarg.h>
#include <stdio.h>
#include <string.h>
#include <libavcodec/avcodec.h>
#include <libavutil/error.h>
#include <libavutil/log.h>

extern void hdmaGoLog(uintptr_t handle, int level, char *message);
static _Thread_local uintptr_t log_handle;

uintptr_t hdma_log_enter(uintptr_t handle)
{
    uintptr_t previous = log_handle;
    log_handle = handle;
    return previous;
}

void hdma_log_leave(uintptr_t previous) { log_handle = previous; }

static void capture_log(void *ptr, int level, const char *format, va_list args)
{
    if (level > av_log_get_level())
        return;
    va_list copy;
    va_copy(copy, args);
    av_log_default_callback(ptr, level, format, copy);
    va_end(copy);
    if (level <= AV_LOG_WARNING) {
        char message[2048];
        va_copy(copy, args);
        int size = vsnprintf(message, sizeof(message), format, copy);
        va_end(copy);
        if (size < 0)
            snprintf(message, sizeof(message), "FFmpeg log message could not be formatted");
        else if (size >= (int)sizeof(message))
            memcpy(message + sizeof(message) - 16, "...[truncated]", 15);
        message[sizeof(message) - 1] = 0;
        hdmaGoLog(log_handle, level, message);
    }
}

void hdma_init_logging(void) { av_log_set_callback(capture_log); }

const char *hdma_error_kind(int code)
{
    if (code == AVERROR(EINTR) || code == AVERROR(EAGAIN) || code == AVERROR(ETIMEDOUT))
        return "temporary_io";
    if (code == AVERROR(EACCES) || code == AVERROR(EPERM) || code == AVERROR(EROFS))
        return "permission";
    if (code == AVERROR(ENOSPC)) return "no_space";
    if (code == AVERROR(ENOENT)) return "not_found";
    if (code == AVERROR(EIO)) return "io";
    if (code == AVERROR(ENOMEM)) return "memory";
    if (code == AVERROR_INVALIDDATA) return "corrupt_data";
    if (code == AVERROR_STREAM_NOT_FOUND) return "track_missing";
    if (code == AVERROR_DECODER_NOT_FOUND || code == AVERROR_ENCODER_NOT_FOUND ||
        code == AVERROR_DEMUXER_NOT_FOUND || code == AVERROR_MUXER_NOT_FOUND)
        return "codec_unavailable";
    return "native_failed";
}
