#include "native.h"
#include <errno.h>
#include <limits.h>
#include <stdio.h>
#include <string.h>

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/audio_fifo.h>
#include <libavutil/error.h>
#include <libavutil/intreadwrite.h>
#include <libavutil/opt.h>
#include <libswresample/swresample.h>

extern int hdmaGoCancelled(uintptr_t handle);
extern int hdmaGoPCM(uintptr_t handle, unsigned char *data, int size);

/* One call owns all of these resources; zero initialization makes a single
 * cleanup path safe even when setup fails partway through. No codec contexts
 * are shared between calls. stage provides context for native error codes. */
typedef struct Operation {
    uintptr_t handle;
    AVFormatContext *input;
    AVCodecContext *decoder;
    AVFormatContext *output;
    AVCodecContext *encoder;
    AVAudioFifo *fifo;
    SwrContext *swr;
    AVChannelLayout layout;
    enum AVSampleFormat input_format;
    int rate;
    int bits;
    int stream_index;
    int64_t pts;
    int64_t samples;
    const char *stage;
} Operation;

const char *hdma_version(void) { return av_version_info(); }

int hdma_check(void)
{
    return !(avcodec_find_decoder(AV_CODEC_ID_DTS) &&
             avcodec_find_decoder(AV_CODEC_ID_TRUEHD) &&
             avcodec_find_encoder(AV_CODEC_ID_TRUEHD) &&
             avcodec_find_decoder(AV_CODEC_ID_EAC3) &&
             avcodec_find_encoder(AV_CODEC_ID_EAC3) &&
             av_guess_format("truehd", NULL, NULL) &&
             av_guess_format("eac3", NULL, NULL) &&
             av_find_input_format("matroska"));
}

static int cancelled(void *opaque)
{
    Operation *op = opaque;
    return hdmaGoCancelled(op->handle);
}

static int fail(char *message, const char *stage, int ret)
{
    char detail[AV_ERROR_MAX_STRING_SIZE];
    av_strerror(ret, detail, sizeof(detail));
    snprintf(message, 512, "%s: %s", stage, detail);
    return ret;
}

static void close_operation(Operation *op)
{
    /* avformat_close_input owns input I/O, whereas output I/O must be closed
     * explicitly before freeing its format context. */
    swr_free(&op->swr);
    av_audio_fifo_free(op->fifo);
    avcodec_free_context(&op->encoder);
    avcodec_free_context(&op->decoder);
    avformat_close_input(&op->input);
    if (op->output) {
        if (op->output->pb)
            avio_closep(&op->output->pb);
        avformat_free_context(op->output);
    }
    av_channel_layout_uninit(&op->layout);
}

static int open_container(Operation *op, const char *path)
{
    int ret;
    op->stage = "open input";
    if (cancelled(op))
        return AVERROR_EXIT;
    op->input = avformat_alloc_context();
    if (!op->input)
        return AVERROR(ENOMEM);
    op->input->interrupt_callback = (AVIOInterruptCB){cancelled, op};
    /* Keep all media I/O local, and never turn a corrupt packet into a
     * success-shaped decode with concealment standing in for lost PCM. */
    AVDictionary *options = NULL;
    av_dict_set(&options, "protocol_whitelist", "file", 0);
    av_dict_set(&options, "err_detect", "explode", 0);
    ret = avformat_open_input(&op->input, path, NULL, &options);
    av_dict_free(&options);
    if (ret < 0)
        return ret;
    op->stage = "read stream information";
    if ((ret = avformat_find_stream_info(op->input, NULL)) < 0)
        return ret;
    return cancelled(op) ? AVERROR_EXIT : 0;
}

static int open_input(Operation *op, const char *path, int index)
{
    int ret = open_container(op, path);
    if (ret < 0)
        return ret;
    op->stage = "select audio stream (containers require an explicit stream index)";
    /* Preserve the original single-stream contract when no index is supplied.
     * An explicit selection never falls back to another/default audio track. */
    if (index < 0) {
        if (op->input->nb_streams != 1)
            return AVERROR_INVALIDDATA;
        index = 0;
    }
    if ((unsigned)index >= op->input->nb_streams ||
        op->input->streams[index]->codecpar->codec_type != AVMEDIA_TYPE_AUDIO)
        return AVERROR_INVALIDDATA;
    op->stream_index = index;
    AVCodecParameters *par = op->input->streams[index]->codecpar;
    if (par->codec_id == AV_CODEC_ID_NONE || par->sample_rate <= 0 ||
        !av_channel_layout_check(&par->ch_layout))
        return AVERROR_INVALIDDATA;
    return cancelled(op) ? AVERROR_EXIT : 0;
}

static int describe_stream(AVStream *stream, HDMAInfo *info)
{
    AVCodecParameters *par = stream->codecpar;
    const char *profile = avcodec_profile_name(par->codec_id, par->profile);
    info->index = stream->index;
    info->rate = par->sample_rate;
    info->bits = par->bits_per_raw_sample ? par->bits_per_raw_sample : par->bits_per_coded_sample;
    info->channels = par->ch_layout.nb_channels;
    snprintf(info->codec, sizeof(info->codec), "%s", avcodec_get_name(par->codec_id));
    snprintf(info->profile, sizeof(info->profile), "%s", profile ? profile : "");
    return av_channel_layout_describe(&par->ch_layout, info->layout, sizeof(info->layout));
}

int hdma_probe_stream(const char *path, int index, uintptr_t handle, HDMAInfo *info, char *error)
{
    Operation op = {.handle = handle};
    int ret = open_input(&op, path, index);
    if (ret >= 0)
        ret = describe_stream(op.input->streams[op.stream_index], info);
    if (ret < 0)
        fail(error, op.stage, ret);
    close_operation(&op);
    return ret;
}

int hdma_probe(const char *path, uintptr_t handle, HDMAInfo *info, char *error)
{
    return hdma_probe_stream(path, -1, handle, info, error);
}

void hdma_tracks_free(HDMATrack *tracks) { av_free(tracks); }

int hdma_probe_tracks(const char *path, uintptr_t handle, HDMATrack **tracks,
                      int *count, char *error)
{
    Operation op = {.handle = handle};
    *tracks = NULL;
    *count = 0;
    int ret = open_container(&op, path);
    if (ret < 0)
        goto end;
    op.stage = "enumerate audio streams";
    if (op.input->nb_streams > INT_MAX) {
        ret = AVERROR_INVALIDDATA;
        goto end;
    }
    *tracks = av_calloc(op.input->nb_streams, sizeof(**tracks));
    if (!*tracks) {
        ret = AVERROR(ENOMEM);
        goto end;
    }
    for (unsigned i = 0; i < op.input->nb_streams; i++) {
        AVStream *stream = op.input->streams[i];
        if (stream->codecpar->codec_type != AVMEDIA_TYPE_AUDIO)
            continue;
        HDMATrack *track = &(*tracks)[(*count)++];
        if ((ret = describe_stream(stream, &track->info)) < 0)
            goto end;
        AVDictionaryEntry *language = av_dict_get(stream->metadata, "language", NULL, 0);
        AVDictionaryEntry *title = av_dict_get(stream->metadata, "title", NULL, 0);
        snprintf(track->language, sizeof(track->language), "%s", language ? language->value : "");
        snprintf(track->title, sizeof(track->title), "%s", title ? title->value : "");
        track->is_default = !!(stream->disposition & AV_DISPOSITION_DEFAULT);
        track->duration_us = stream->duration != AV_NOPTS_VALUE ?
            av_rescale_q(stream->duration, stream->time_base, AV_TIME_BASE_Q) :
            op.input->duration != AV_NOPTS_VALUE ? op.input->duration : 0;
    }
    if (!*count)
        ret = AVERROR_STREAM_NOT_FOUND;
end:
    if (ret < 0) {
        hdma_tracks_free(*tracks);
        *tracks = NULL;
        *count = 0;
        fail(error, op.stage, ret);
    }
    close_operation(&op);
    return ret;
}

static int open_decoder(Operation *op, const HDMAInfo *expected)
{
    AVCodecParameters *par = op->input->streams[op->stream_index]->codecpar;
    const AVCodec *codec = avcodec_find_decoder(par->codec_id);
    int ret;
    op->stage = "open decoder / validate input format";
    if (!codec)
        return AVERROR_DECODER_NOT_FOUND;
    if ((ret = av_channel_layout_from_string(&op->layout, expected->layout)) < 0)
        return ret;
    /* Probe and processing reopen the file separately. Refuse changes to the
     * expected stream identity/format rather than encoding under stale labels. */
    if (expected->index != op->input->streams[op->stream_index]->index ||
        expected->rate != par->sample_rate || expected->channels != par->ch_layout.nb_channels ||
        av_channel_layout_compare(&op->layout, &par->ch_layout))
        return AVERROR_INVALIDDATA;
    if (expected->codec[0] && strcmp(expected->codec, avcodec_get_name(par->codec_id)))
        return AVERROR_INVALIDDATA;
    const char *profile = avcodec_profile_name(par->codec_id, par->profile);
    if (expected->profile[0] && (!profile || strcmp(expected->profile, profile)))
        return AVERROR_INVALIDDATA;
    op->rate = expected->rate;
    op->bits = expected->bits;
    op->decoder = avcodec_alloc_context3(codec);
    if (!op->decoder)
        return AVERROR(ENOMEM);
    if ((ret = avcodec_parameters_to_context(op->decoder, par)) < 0)
        return ret;
    op->decoder->err_recognition = AV_EF_EXPLODE | AV_EF_CRCCHECK | AV_EF_BITSTREAM | AV_EF_BUFFER;
    /* core_only would silently discard the lossless extension. DTS:X still
     * yields compatible channel PCM here; this is not a spatial renderer. */
    if (par->codec_id == AV_CODEC_ID_DTS &&
        (ret = av_opt_set_int(op->decoder->priv_data, "core_only", 0, 0)) < 0)
        return ret;
    return avcodec_open2(op->decoder, codec, NULL);
}

static int open_encoder(Operation *op, const char *output)
{
    const AVCodec *codec = avcodec_find_encoder(AV_CODEC_ID_TRUEHD);
    AVStream *stream;
    int ret;
    op->stage = "open TrueHD encoder";
    if (!codec)
        return AVERROR_ENCODER_NOT_FOUND;
    op->encoder = avcodec_alloc_context3(codec);
    if (!op->encoder)
        return AVERROR(ENOMEM);
    op->encoder->sample_rate = op->rate;
    /* The encoder consumes planar integers. Twenty-four effective bits occupy
     * the high bits of int32 samples; no float quantization is introduced. */
    op->encoder->sample_fmt = op->bits == 16 ? AV_SAMPLE_FMT_S16P : AV_SAMPLE_FMT_S32P;
    op->encoder->time_base = (AVRational){1, op->rate};
    op->encoder->strict_std_compliance = FF_COMPLIANCE_EXPERIMENTAL;
    if ((ret = av_channel_layout_copy(&op->encoder->ch_layout, &op->layout)) < 0 ||
        (ret = avcodec_open2(op->encoder, codec, NULL)) < 0)
        return ret;
    /* DTS decoder frames and TrueHD access units have different lengths.
     * FIFO reblocking preserves every sample without imposing decoder framing
     * on the encoder. The final short block is handled during flush. */
    op->fifo = av_audio_fifo_alloc(op->encoder->sample_fmt, op->layout.nb_channels, op->encoder->frame_size);
    if (!op->fifo)
        return AVERROR(ENOMEM);
    op->stage = "create raw TrueHD output";
    if ((ret = avformat_alloc_output_context2(&op->output, NULL, "truehd", output)) < 0)
        return ret;
    op->output->interrupt_callback = (AVIOInterruptCB){cancelled, op};
    stream = avformat_new_stream(op->output, NULL);
    if (!stream)
        return AVERROR(ENOMEM);
    stream->time_base = op->encoder->time_base;
    if ((ret = avcodec_parameters_from_context(stream->codecpar, op->encoder)) < 0 ||
        (ret = avio_open2(&op->output->pb, output, AVIO_FLAG_WRITE, &op->output->interrupt_callback, NULL)) < 0)
        return ret;
    return avformat_write_header(op->output, NULL);
}

static int write_packets(Operation *op)
{
    AVPacket *packet = av_packet_alloc();
    int ret;
    if (!packet)
        return AVERROR(ENOMEM);
    while ((ret = avcodec_receive_packet(op->encoder, packet)) >= 0) {
        av_packet_rescale_ts(packet, op->encoder->time_base, op->output->streams[0]->time_base);
        packet->stream_index = 0;
        ret = av_interleaved_write_frame(op->output, packet);
        av_packet_unref(packet);
        if (ret < 0 || cancelled(op)) {
            if (ret >= 0)
                ret = AVERROR_EXIT;
            break;
        }
    }
    av_packet_free(&packet);
    /* EAGAIN means the encoder needs more PCM; EOF is normal after flush.
     * Other errors, including output I/O and cancellation, propagate. */
    return ret == AVERROR(EAGAIN) || ret == AVERROR_EOF ? 0 : ret;
}

static int encode_fifo(Operation *op, int final)
{
    int ret = 0, available;
    while ((available = av_audio_fifo_size(op->fifo)) >= op->encoder->frame_size || (final && available)) {
        AVFrame *frame = av_frame_alloc();
        if (!frame)
            return AVERROR(ENOMEM);
        /* Never zero-pad the last unit to frame_size: verification requires
         * the encoded stream to retain the exact source sample count. */
        frame->nb_samples = FFMIN(available, op->encoder->frame_size);
        frame->sample_rate = op->rate;
        frame->format = op->encoder->sample_fmt;
        frame->pts = op->pts;
        op->pts += frame->nb_samples;
        ret = av_channel_layout_copy(&frame->ch_layout, &op->layout);
        if (ret >= 0)
            ret = av_frame_get_buffer(frame, 0);
        if (ret >= 0 && av_audio_fifo_read(op->fifo, (void **)frame->extended_data, frame->nb_samples) != frame->nb_samples)
            ret = AVERROR_BUG;
        if (ret >= 0)
            ret = avcodec_send_frame(op->encoder, frame);
        av_frame_free(&frame);
        if (ret < 0 || (ret = write_packets(op)) < 0)
            return ret;
    }
    return ret;
}

static int consume_frame(Operation *op, AVFrame *frame)
{
    int ret, count, channels = op->layout.nb_channels;
    uint8_t *pcm = NULL;
    AVFrame *planar = NULL;
    op->stage = "validate decoded frame / convert sample representation";
    if (cancelled(op))
        return AVERROR_EXIT;
    /* Headers alone are insufficient: reject damaged frames or midstream
     * format changes before any PCM reaches hashing or encoding. */
    if (frame->flags & AV_FRAME_FLAG_CORRUPT || frame->decode_error_flags ||
        frame->sample_rate != op->rate || av_channel_layout_compare(&frame->ch_layout, &op->layout))
        return AVERROR_INVALIDDATA;
    if (op->decoder->codec_id == AV_CODEC_ID_DTS &&
        op->decoder->profile != AV_PROFILE_DTS_HD_MA &&
        op->decoder->profile != AV_PROFILE_DTS_HD_MA_X &&
        op->decoder->profile != AV_PROFILE_DTS_HD_MA_X_IMAX) {
        op->stage = "validate lossless DTS-HD MA / DTS:X decoder profile";
        return AVERROR_INVALIDDATA;
    }
    if (!op->swr) {
        /* swresample only normalizes sample representation. Equal input/output
         * rates and layouts plus disabled dither prohibit intentional remixing,
         * resampling or added noise. The checks below enforce this invariant. */
        op->input_format = frame->format;
        ret = swr_alloc_set_opts2(&op->swr, &op->layout, AV_SAMPLE_FMT_S32, op->rate,
                                 &op->layout, frame->format, op->rate, 0, NULL);
        if (ret < 0 || (ret = av_opt_set_int(op->swr, "dither_method", SWR_DITHER_NONE, 0)) < 0 ||
            (ret = swr_init(op->swr)) < 0)
            return ret;
    } else if (frame->format != op->input_format) {
        return AVERROR_INVALIDDATA;
    }
    if (frame->nb_samples <= 0 || frame->nb_samples > INT_MAX / (channels * 4))
        return AVERROR_INVALIDDATA;
    pcm = av_malloc((size_t)frame->nb_samples * channels * 4);
    if (!pcm)
        return AVERROR(ENOMEM);
    count = swr_convert(op->swr, &pcm, frame->nb_samples,
                        (const uint8_t **)frame->extended_data, frame->nb_samples);
    /* Representation conversion must be one-to-one and introduce no delay.
     * Buffered samples would otherwise require a separate flush path. */
    if (count != frame->nb_samples || swr_get_delay(op->swr, op->rate) != 0) {
        ret = count < 0 ? count : AVERROR_INVALIDDATA;
        goto end;
    }
    /* Check unused low bits before narrowing or hashing. A 32-bit container
     * must not hide precision beyond the declared 16/24 effective bits.
     * Explicit full-width lossy validation has no zero-padding invariant. */
    for (int i = 0; i < count * channels; i++) {
        uint32_t mask = op->bits == 32 ? 0 : op->bits == 16 ? 0xffff : 0xff;
        if (((uint32_t *)pcm)[i] & mask) {
            op->stage = "decoded PCM exceeds declared effective precision";
            ret = AVERROR_INVALIDDATA;
            goto end;
        }
    }
    if (!op->encoder) {
        /* The callback contract is explicitly little-endian int32, independent
         * of host byte order. Go consumes this borrowed buffer synchronously. */
        for (int i = 0; i < count * channels; i++) {
            uint32_t sample = ((uint32_t *)pcm)[i];
            AV_WL32(pcm + 4*i, sample);
        }
        ret = hdmaGoPCM(op->handle, pcm, count * channels * 4) < 0 ? AVERROR_EXTERNAL : 0;
        goto end;
    }
    /* Encoding retains native integer values and speaker order; only
     * interleaved-to-planar storage is changed before FIFO reblocking. */
    planar = av_frame_alloc();
    if (!planar) {
        ret = AVERROR(ENOMEM);
        goto end;
    }
    planar->nb_samples = count;
    planar->format = op->encoder->sample_fmt;
    if ((ret = av_channel_layout_copy(&planar->ch_layout, &op->layout)) < 0 ||
        (ret = av_frame_get_buffer(planar, 0)) < 0)
        goto end;
    for (int ch = 0; ch < channels; ch++) {
        for (int i = 0; i < count; i++) {
            int32_t sample = ((int32_t *)pcm)[i * channels + ch];
            if (op->bits == 16)
                ((int16_t *)planar->extended_data[ch])[i] = sample >> 16;
            else
                ((int32_t *)planar->extended_data[ch])[i] = sample;
        }
    }
    if (av_audio_fifo_write(op->fifo, (void **)planar->extended_data, count) != count) {
        ret = AVERROR(ENOMEM);
        goto end;
    }
    op->stage = "encode TrueHD frame";
    ret = encode_fifo(op, 0);
end:
    if (ret >= 0)
        op->samples += count;
    av_frame_free(&planar);
    av_free(pcm);
    return ret;
}

static int receive_frames(Operation *op)
{
    AVFrame *frame = av_frame_alloc();
    int ret;
    if (!frame)
        return AVERROR(ENOMEM);
    while ((ret = avcodec_receive_frame(op->decoder, frame)) >= 0) {
        ret = consume_frame(op, frame);
        av_frame_unref(frame);
        if (ret < 0)
            break;
    }
    av_frame_free(&frame);
    return ret == AVERROR(EAGAIN) || ret == AVERROR_EOF ? 0 : ret;
}

int hdma_process(const char *path, const char *output, const HDMAInfo *expected,
                 uintptr_t handle, char *error)
{
    Operation op = {.handle = handle};
    AVPacket *packet = NULL;
    int ret = open_input(&op, path, expected->index);
    if (ret < 0 || (ret = open_decoder(&op, expected)) < 0)
        goto end;
    /* NULL output selects decode/hash callbacks. Otherwise Go supplies a
     * private staging path and owns validation/publication after this returns. */
    if (output && (ret = open_encoder(&op, output)) < 0)
        goto end;
    packet = av_packet_alloc();
    if (!packet) {
        ret = AVERROR(ENOMEM);
        goto end;
    }
    op.stage = "decode input";
    while ((ret = av_read_frame(op.input, packet)) >= 0) {
        if (cancelled(&op)) {
            ret = AVERROR_EXIT;
            goto end;
        }
        if (packet->stream_index == expected->index && (packet->flags & AV_PKT_FLAG_CORRUPT)) {
            ret = AVERROR_INVALIDDATA;
            goto end;
        }
        if (packet->stream_index == expected->index) {
            /* Drain all decoded frames from each packet before submitting the
             * next one, so decoder backpressure is handled synchronously. */
            ret = avcodec_send_packet(op.decoder, packet);
            if (ret < 0 || (ret = receive_frames(&op)) < 0)
                goto end;
        }
        av_packet_unref(packet);
        op.stage = "read next input packet";
    }
    /* Only a clean demux EOF permits flushing. Do not ignore a read error
     * merely because its successfully decoded prefix is nonempty. */
    if (ret != AVERROR_EOF)
        goto end;
    op.stage = "flush decoder";
    if ((ret = avcodec_send_packet(op.decoder, NULL)) < 0 ||
        (ret = receive_frames(&op)) < 0)
        goto end;
    if (!op.samples) {
        ret = AVERROR_INVALIDDATA;
        goto end;
    }
    if (op.encoder) {
        /* Flush in dependency order: decoded tail, FIFO remainder, delayed
         * encoder packets, then muxer trailer. The patched encoder handles
         * streams shorter than its initial restart interval. */
        op.stage = "flush TrueHD encoder";
        if ((ret = encode_fifo(&op, 1)) < 0 ||
            (ret = avcodec_send_frame(op.encoder, NULL)) < 0 ||
            (ret = write_packets(&op)) < 0)
            goto end;
        op.stage = "finalize TrueHD output";
        if ((ret = av_write_trailer(op.output)) < 0)
            goto end;
        avio_flush(op.output->pb);
        if ((ret = op.output->pb->error) < 0)
            goto end;
        ret = avio_closep(&op.output->pb);
    }
end:
    if (ret < 0)
        fail(error, op.stage, ret);
    av_packet_free(&packet);
    close_operation(&op);
    return ret;
}
