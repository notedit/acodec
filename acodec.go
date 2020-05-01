package acodec

/*
#cgo LDFLAGS: -lavformat -lavutil -lavcodec -lswresample
#cgo CFLAGS: -Wno-deprecated
#include <libavformat/avformat.h>
#include <libavcodec/avcodec.h>
#include <libavutil/avutil.h>
#include <libswresample/swresample.h>
#include <libavutil/opt.h>
#include <string.h>


typedef struct {
	AVCodec *codec;
	AVCodecContext *codecCtx;
	AVFrame *frame;
	AVDictionary *options;
	int profile;
} FFCodec;

int wrap_avcodec_decode_audio4(AVCodecContext *ctx, AVFrame *frame, void *data, int size, int *got) {
	struct AVPacket pkt = {.data = data, .size = size};
	return avcodec_decode_audio4(ctx, frame, got, &pkt);
}

int wrap_swresample_convert(SwrContext *avr, int *out, int outcount, int *in,  int incount) {
	return swr_convert(avr, (void *)out, outcount, (void *)in, incount);
}


*/
import "C"
import (
	"fmt"
	"runtime"
	"time"
	"unsafe"
)

type ffctx struct {
	ff C.FFCodec
}

func newFFCtxByCodec(codec *C.AVCodec) (ff *ffctx, err error) {
	ff = &ffctx{}
	ff.ff.codec = codec
	ff.ff.codecCtx = C.avcodec_alloc_context3(codec)
	ff.ff.profile = C.FF_PROFILE_UNKNOWN
	runtime.SetFinalizer(ff, freeFFCtx)
	return
}

func freeFFCtx(self *ffctx) {
	ff := &self.ff
	if ff.frame != nil {
		C.av_frame_free(&ff.frame)
	}
	if ff.codecCtx != nil {
		C.avcodec_close(ff.codecCtx)
		C.av_free(unsafe.Pointer(ff.codecCtx))
		ff.codecCtx = nil
	}
	if ff.options != nil {
		C.av_dict_free(&ff.options)
	}
}



type SampleFormat uint8

const (
	U8   = SampleFormat(iota + 1) // 8-bit unsigned integer
	S16                           // signed 16-bit integer
	S32                           // signed 32-bit integer
	FLT                           // 32-bit float
	DBL                           // 64-bit float
	U32                           // unsigned 32-bit integer
)

func (self SampleFormat) BytesPerSample() int {
	switch self {
	case U8:
		return 1
	case S16:
		return 2
	case FLT, S32, U32:
		return 4
	case DBL:
		return 8
	default:
		return 0
	}
}

type AudioFrame struct {
	SampleFormat SampleFormat // audio sample format, e.g: S16,FLTP,...
	Channels     int          // audio channel layout, e.g: CH_MONO,CH_STEREO,...
	SampleCount  int          // sample count in this frame
	SampleRate   int          // sample rate
	Data         [][]byte     // data array for planar format len(Data) > 1
}


type ATranscorder struct {
	encoder         *ffctx
	decoder         *ffctx
	framebuf        []byte
	resample        *C.SwrContext
	InSampleRate    int
	InChannels      int
	OutSampleRate   int
	OutChannels     int
	OutBitrate      int
	EncodeCodecName string
	DecodeCodecName string
}

func (self *ATranscorder) Setup() (err error) {

	err = self.initdecoder()
	if err != nil {
		return
	}

	ff := &self.decoder.ff

	ff.frame = C.av_frame_alloc()
	ff.frame.format = C.int(C.AV_SAMPLE_FMT_S16)
	ff.frame.channel_layout = channel2ChannelLayout(self.InChannels)
	ff.frame.sample_rate = C.int(self.InSampleRate)
	ff.frame.channels = C.int(self.InChannels)


	ff.codecCtx.sample_rate = C.int(self.InSampleRate)
	ff.codecCtx.channel_layout = channel2ChannelLayout(self.InChannels)
	ff.codecCtx.channels = C.int(self.InChannels)


	if C.avcodec_open2(ff.codecCtx, ff.codec, nil) != 0 {
		err = fmt.Errorf("ffmpeg: decoder: avcodec_open2 failed")
		return
	}

	ff.frame.nb_samples = C.int(ff.codecCtx.frame_size)

	fmt.Println("decode frame size ",int(ff.codecCtx.frame_size))

	err = self.initencoder()
	if err != nil {
		return
	}

	ff = &self.encoder.ff
	ff.frame = C.av_frame_alloc()

	ff.frame.format = C.int(C.AV_SAMPLE_FMT_S16)
	ff.frame.channel_layout = channel2ChannelLayout(self.OutChannels)
	ff.frame.sample_rate = C.int(self.OutSampleRate)
	ff.frame.channels = C.int(self.OutChannels)


	ff.codecCtx.sample_fmt = C.AV_SAMPLE_FMT_S16
	ff.codecCtx.sample_rate = C.int(self.OutSampleRate)
	ff.codecCtx.bit_rate = C.int64_t(self.OutBitrate)
	ff.codecCtx.channel_layout = channel2ChannelLayout(self.OutChannels)

	if C.avcodec_open2(ff.codecCtx, ff.codec, nil) != 0 {
		err = fmt.Errorf("ffmpeg: encoder: avcodec_open2 failed")
		return
	}

	ff.frame.nb_samples = C.int(ff.codecCtx.frame_size)

	fmt.Println("encode frame size ",int(ff.codecCtx.frame_size))

	err = self.initresample()
	if err != nil {
		return
	}

	return
}

func (self *ATranscorder) initresample() error {

	swrCtx := C.swr_alloc()
	C.av_opt_set_int(unsafe.Pointer(swrCtx), C.CString("in_channel_layout"), C.int64_t(channel2ChannelLayout(self.InChannels)), 0)
	C.av_opt_set_int(unsafe.Pointer(swrCtx), C.CString("in_sample_rate"), C.int64_t(self.InSampleRate), 0)
	C.av_opt_set_int(unsafe.Pointer(swrCtx), C.CString("in_sample_fmt"), C.int64_t(C.AV_SAMPLE_FMT_S16), 0)

	C.av_opt_set_int(unsafe.Pointer(swrCtx), C.CString("out_channel_layout"), C.int64_t(channel2ChannelLayout(self.OutChannels)), 0)
	C.av_opt_set_int(unsafe.Pointer(swrCtx), C.CString("out_sample_rate"), C.int64_t(self.InSampleRate), 0)
	C.av_opt_set_int(unsafe.Pointer(swrCtx), C.CString("out_sample_fmt"), C.int64_t(C.AV_SAMPLE_FMT_S16), 0)
	C.swr_init(swrCtx)
	self.resample = swrCtx

	return nil
}

func (self *ATranscorder) initencoder() (err error) {

	codec := C.avcodec_find_encoder_by_name(C.CString(self.EncodeCodecName))
	if codec == nil || C.avcodec_get_type(codec.id) != C.AVMEDIA_TYPE_AUDIO {
		err = fmt.Errorf("ffmpeg: cannot find audio encoder name=%s", self.EncodeCodecName)
		return
	}

	if self.encoder, err = newFFCtxByCodec(codec); err != nil {
		return
	}
	return
}

func (self *ATranscorder) initdecoder() (err error) {

	codec := C.avcodec_find_encoder_by_name(C.CString(self.EncodeCodecName))
	if codec == nil || C.avcodec_get_type(codec.id) != C.AVMEDIA_TYPE_AUDIO {
		err = fmt.Errorf("ffmpeg: cannot find audio encoder name=%s", self.EncodeCodecName)
		return
	}

	if self.encoder, err = newFFCtxByCodec(codec); err != nil {
		return
	}
	return
}

func (self *ATranscorder) Do(in []byte) (pkts [][]byte, got bool, err error) {

	// decode
	deff := &self.decoder.ff

	cgotframe := C.int(0)
	cerr := C.wrap_avcodec_decode_audio4(deff.codecCtx, deff.frame, unsafe.Pointer(&in[0]), C.int(len(in)), &cgotframe)
	if cerr < C.int(0) {
		err = fmt.Errorf("ffmpeg: avcodec_decode_audio4 failed: %d", cerr)
		return
	}

	enff := &self.encoder.ff

	// resample
	outSampleCount := int(C.swr_get_out_samples(self.resample, C.int(deff.frame.nb_samples)))

	fmt.Println("out sample count ", outSampleCount)

	cerr = C.swr_convert_frame(self.resample, nil, deff.frame)
	if cerr < C.int(0) {
		err = fmt.Errorf("ffmpeg: swr_convert_frame failed: %d", cerr)
		return
	}

	for {
		remainingSamples := int(C.swr_get_delay(self.resample, C.int64_t(self.OutSampleRate)))
		if remainingSamples < int(enff.codecCtx.frame_size) {
			return
		}

		cerr = C.swr_convert_frame(self.resample, enff.frame, nil)
		if cerr < C.int(0) {
			err = fmt.Errorf("ffmpeg: swr_convert_frame failed: %d", cerr)
			return
		}

		cpkt := C.AVPacket{}
		cgotpkt := C.int(0)
		cerr := C.avcodec_encode_audio2(enff.codecCtx, &cpkt, enff.frame, &cgotpkt)
		if cerr < C.int(0) {
			err = fmt.Errorf("ffmpeg: avcodec_encode_audio2 failed: %d", cerr)
			return
		}

		if cgotpkt != 0 {
			got = true
			pkt := C.GoBytes(unsafe.Pointer(cpkt.data), cpkt.size)
			pkts = append(pkts, pkt)
			C.av_packet_unref(&cpkt)
		}
	}

	return
}

func (self *ATranscorder) Close() {

	freeFFCtx(self.decoder)
	freeFFCtx(self.encoder)
	C.swr_free(&self.resample)
}


// InPacketDuration, for test
func (self *ATranscorder) InPacketDuration(data []byte) (dur time.Duration) {
	ff := &self.decoder.ff
	duration := C.av_get_audio_frame_duration(ff.codecCtx, C.int(len(data)))
	dur = time.Duration(int(duration)) * time.Second / time.Duration(self.InSampleRate)
	return
}

// OutPacketDuration, for test
func (self *ATranscorder) OutPacketDuration(data []byte) (dur time.Duration) {
	ff := &self.encoder.ff
	duration := C.av_get_audio_frame_duration(ff.codecCtx, C.int(len(data)))
	dur = time.Duration(int(duration)) * time.Second / time.Duration(self.InSampleRate)
	return
}


func channel2ChannelLayout(channel int) (layout C.uint64_t) {
	if channel == 1 {
		return C.uint64_t(C.AV_CH_LAYOUT_MONO)
	}
	if channel == 2 {
		return C.uint64_t(C.AV_CH_LAYOUT_STEREO)
	}
	return 0
}
