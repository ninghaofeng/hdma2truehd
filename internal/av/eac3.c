#include "native.h"
#include <errno.h>
#include <string.h>

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/audio_fifo.h>
#include <libavutil/error.h>
#include <libavutil/intreadwrite.h>
#include <libavutil/opt.h>

extern int hdmaGoCancelled(uintptr_t handle);

struct HDMAEAC3 {
    uintptr_t handle;
    AVCodecContext *encoders[2];
    AVFormatContext *output;
    AVAudioFifo *fifo;
    AVChannelLayout layout;
    int streams;
    int channels;
    int64_t frames;
    uint64_t samples;
};

static int report(char *error, const char *stage, int ret)
{
    char detail[AV_ERROR_MAX_STRING_SIZE];
    av_strerror(ret, detail, sizeof(detail));
    snprintf(error, 512, "%s: %s", stage, detail);
    return ret;
}

static int interrupted(void *opaque)
{
    HDMAEAC3 *op = opaque;
    return hdmaGoCancelled(op->handle);
}

void hdma_eac3_close(HDMAEAC3 *op)
{
    if (!op)
        return;
    for (int i = 0; i < 2; i++)
        avcodec_free_context(&op->encoders[i]);
    av_audio_fifo_free(op->fifo);
    av_channel_layout_uninit(&op->layout);
    if (op->output) {
        if (op->output->pb)
            avio_closep(&op->output->pb);
        avformat_free_context(op->output);
    }
    av_free(op);
}

int hdma_eac3_open(const char *path, int channels, int bitrate,
                   uintptr_t handle, HDMAEAC3 **result, char *error)
{
    *result = NULL;
    if ((channels != 6 && channels != 8) || bitrate % 32000 ||
        bitrate < (channels == 8 ? 768000 : 384000) ||
        bitrate > (channels == 8 ? 1536000 : 1024000))
        return report(error, "E-AC-3 configuration", AVERROR(EINVAL));
    HDMAEAC3 *op = av_mallocz(sizeof(*op));
    if (!op)
        return report(error, "allocate E-AC-3 operation", AVERROR(ENOMEM));
    op->handle = handle;
    op->channels = channels;
    op->streams = channels == 8 ? 2 : 1;
    int ret = av_channel_layout_from_string(&op->layout, channels == 8 ? "7.1" : "5.1(side)");
    if (ret < 0)
        goto failed;
    const AVCodec *codec = avcodec_find_encoder(AV_CODEC_ID_EAC3);
    if (!codec) {
        ret = AVERROR_ENCODER_NOT_FOUND;
        goto failed;
    }
    for (int i = 0; i < op->streams; i++) {
        AVCodecContext *ctx = op->encoders[i] = avcodec_alloc_context3(codec);
        if (!ctx) {
            ret = AVERROR(ENOMEM);
            goto failed;
        }
        ctx->sample_rate = 48000;
        ctx->sample_fmt = AV_SAMPLE_FMT_FLTP;
        ctx->time_base = (AVRational){1, 48000};
        ctx->bit_rate = channels == 6 ? bitrate : bitrate * (i ? 3 : 5) / 8;
        ret = av_channel_layout_from_string(&ctx->ch_layout, i ? "quad" : "5.1(side)");
        if (ret < 0)
            goto failed;
        if (i && ((ret = av_opt_set_int(ctx->priv_data, "dependent_71", 1, 0)) < 0 ||
                  (ret = av_opt_set_int(ctx->priv_data, "channel_coupling", 0, 0)) < 0))
            goto failed;
        if ((ret = avcodec_open2(ctx, codec, NULL)) < 0)
            goto failed;
        if (ctx->frame_size != 1536 || ctx->initial_padding != 256) {
            ret = AVERROR_INVALIDDATA;
            goto failed;
        }
    }
    op->fifo = av_audio_fifo_alloc(AV_SAMPLE_FMT_FLTP, channels, 1536);
    if (!op->fifo) {
        ret = AVERROR(ENOMEM);
        goto failed;
    }
    if ((ret = avformat_alloc_output_context2(&op->output, NULL, "eac3", path)) < 0)
        goto failed;
    op->output->interrupt_callback = (AVIOInterruptCB){interrupted, op};
    AVStream *stream = avformat_new_stream(op->output, NULL);
    if (!stream) {
        ret = AVERROR(ENOMEM);
        goto failed;
    }
    stream->time_base = (AVRational){1, 48000};
    if ((ret = avcodec_parameters_from_context(stream->codecpar, op->encoders[0])) < 0 ||
        (ret = av_channel_layout_copy(&stream->codecpar->ch_layout, &op->layout)) < 0)
        goto failed;
    stream->codecpar->bit_rate = bitrate;
    if ((ret = avio_open2(&op->output->pb, path, AVIO_FLAG_WRITE,
                         &op->output->interrupt_callback, NULL)) < 0 ||
        (ret = avformat_write_header(op->output, NULL)) < 0)
        goto failed;
    *result = op;
    return 0;
failed:
    hdma_eac3_close(op);
    return report(error, "open E-AC-3 encoder/output", ret);
}

static AVFrame *new_frame(const AVChannelLayout *layout, int count)
{
    AVFrame *frame = av_frame_alloc();
    if (!frame)
        return NULL;
    frame->format = AV_SAMPLE_FMT_FLTP;
    frame->sample_rate = 48000;
    frame->nb_samples = count;
    if (av_channel_layout_copy(&frame->ch_layout, layout) < 0 ||
        av_frame_get_buffer(frame, 0) < 0)
        av_frame_free(&frame);
    return frame;
}

static int encode_block(HDMAEAC3 *op)
{
    AVFrame *source = new_frame(&op->layout, 1536);
    AVPacket *packets[2] = {NULL, NULL};
    AVPacket *combined = NULL;
    int ret = AVERROR(ENOMEM);
    if (!source)
        goto end;
    if (av_audio_fifo_read(op->fifo, (void **)source->extended_data, 1536) != 1536) {
        ret = AVERROR_BUG;
        goto end;
    }
    for (int stream = 0; stream < op->streams; stream++) {
        AVCodecContext *ctx = op->encoders[stream];
        AVFrame *frame = new_frame(&ctx->ch_layout, 1536);
        if (!frame) {
            ret = AVERROR(ENOMEM);
            goto end;
        }
        frame->pts = op->frames * 1536;
        /* The core has a bounded 5.1 compatibility downmix. The dependent
         * stream REPLACES its surrounds and adds the rears, retaining all
         * four original surround channels in the final 7.1 presentation.
         * Dependent 0x1a00 order is side L/R, then back L/R. */
        for (int ch = 0; ch < ctx->ch_layout.nb_channels; ch++) {
            float *target = (float *)frame->extended_data[ch];
            const int map[4] = {6, 7, 4, 5};
            int input = stream ? map[ch] : ch;
            const float *pcm = (const float *)source->extended_data[input];
            if (!stream && op->channels == 8 && ch >= 4) {
                const float *side = (const float *)source->extended_data[ch + 2];
                for (int n = 0; n < 1536; n++)
                    target[n] = 0.5f * (pcm[n] + side[n]);
            } else {
                memcpy(target, pcm, 1536 * sizeof(float));
            }
        }
        ret = avcodec_send_frame(ctx, frame);
        av_frame_free(&frame);
        if (ret < 0)
            goto end;
        packets[stream] = av_packet_alloc();
        if (!packets[stream]) {
            ret = AVERROR(ENOMEM);
            goto end;
        }
        if ((ret = avcodec_receive_packet(ctx, packets[stream])) < 0)
            goto end;
        if (packets[stream]->pts != op->frames * 1536 - 256 ||
            packets[stream]->duration != 1536) {
            ret = AVERROR_INVALIDDATA;
            goto end;
        }
    }
    combined = av_packet_alloc();
    if (!combined) {
        ret = AVERROR(ENOMEM);
        goto end;
    }
    int size = packets[0]->size + (op->streams == 2 ? packets[1]->size : 0);
    if ((ret = av_new_packet(combined, size)) < 0)
        goto end;
    memcpy(combined->data, packets[0]->data, packets[0]->size);
    if (op->streams == 2)
        memcpy(combined->data + packets[0]->size, packets[1]->data, packets[1]->size);
    combined->pts = combined->dts = packets[0]->pts;
    combined->duration = 1536;
    combined->stream_index = 0;
    av_packet_rescale_ts(combined, op->encoders[0]->time_base, op->output->streams[0]->time_base);
    ret = av_interleaved_write_frame(op->output, combined);
    if (ret >= 0)
        op->frames++;
end:
    av_frame_free(&source);
    for (int i = 0; i < 2; i++)
        av_packet_free(&packets[i]);
    av_packet_free(&combined);
    return ret;
}

static int append(HDMAEAC3 *op, const unsigned char *pcm, int count)
{
    AVFrame *frame = new_frame(&op->layout, count);
    if (!frame)
        return AVERROR(ENOMEM);
    for (int ch = 0; ch < op->channels; ch++) {
        float *plane = (float *)frame->extended_data[ch];
        for (int n = 0; n < count; n++)
            plane[n] = pcm ? (int32_t)AV_RL32(pcm + 4 * (n * op->channels + ch)) / 2147483648.0 : 0;
    }
    int ret = av_audio_fifo_write(op->fifo, (void **)frame->extended_data, count);
    av_frame_free(&frame);
    if (ret != count)
        return ret < 0 ? ret : AVERROR_BUG;
    while (av_audio_fifo_size(op->fifo) >= 1536) {
        if (interrupted(op))
            return AVERROR_EXIT;
        if ((ret = encode_block(op)) < 0)
            return ret;
    }
    return 0;
}

int hdma_eac3_write(HDMAEAC3 *op, const unsigned char *pcm, int size, char *error)
{
    if (size <= 0 || size % (op->channels * 4))
        return report(error, "E-AC-3 PCM frame alignment", AVERROR(EINVAL));
    int count = size / (op->channels * 4);
    int ret = append(op, pcm, count);
    if (ret < 0)
        return report(error, "encode E-AC-3 PCM", ret);
    op->samples += count;
    return 0;
}

int hdma_eac3_finish(HDMAEAC3 *op, uint64_t *samples, uint64_t *decoded, char *error)
{
    /* Raw E-AC-3 has no gapless trim metadata. Flush the MDCT's 256-sample
     * delay explicitly so the source tail survives, then pad the final unit.
     * Validation checks the resulting exact length rather than hiding padding. */
    int ret = op->samples ? append(op, NULL, 256) : AVERROR_INVALIDDATA;
    if (ret >= 0 && av_audio_fifo_size(op->fifo))
        ret = append(op, NULL, 1536 - av_audio_fifo_size(op->fifo));
    for (int i = 0; ret >= 0 && i < op->streams; i++) {
        ret = avcodec_send_frame(op->encoders[i], NULL);
        if (ret >= 0) {
            AVPacket *packet = av_packet_alloc();
            if (!packet) {
                ret = AVERROR(ENOMEM);
                break;
            }
            ret = avcodec_receive_packet(op->encoders[i], packet);
            av_packet_free(&packet);
            ret = ret == AVERROR_EOF ? 0 : ret < 0 ? ret : AVERROR_INVALIDDATA;
        }
    }
    if (ret >= 0) ret = av_write_trailer(op->output);
    if (ret >= 0) {
        avio_flush(op->output->pb);
        ret = op->output->pb->error;
    }
    if (ret >= 0) ret = avio_closep(&op->output->pb);
    if (ret < 0)
        return report(error, "finalize E-AC-3 output", ret);
    *samples = op->samples;
    *decoded = op->frames * 1536;
    return 0;
}
