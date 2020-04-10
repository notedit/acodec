package acodec

/*
#cgo LDFLAGS: -lavformat -lavutil -lavcodec -lswresample
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
*/
import "C"
import (
	"fmt"
	"runtime"
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

type ATranscorder struct {
	encoder         *ffctx
	decoder         *ffctx
	resample        *C.SwrContext
	InSampleRate    int
	InChannels      int
	OutSampleRate   int
	OutChannels     int
	OutBitrate      int
	EncodeCodecName string
	DecodeCodecName string
}

func (self *ATranscorder) Setup() error {

	return nil
}

func (self *ATranscorder) initresample() error {

	swr_ctx := C.swr_alloc()
	C.av_opt_set_int(unsafe.Pointer(swr_ctx), C.CString("in_channel_layout"),  channel2ChannelLayout(self.InChannels), 0)
	C.av_opt_set_int(unsafe.Pointer(swr_ctx), C.CString("in_sample_rate"), C.int64_t(self.InSampleRate), 0)
	C.av_opt_set_sample_fmt(unsafe.Pointer(swr_ctx), C.CString("in_sample_fmt"), C.int64_t(C.AV_SAMPLE_FMT_S16), 0)

	C.av_opt_set_int(unsafe.Pointer(swr_ctx), C.CString("out_channel_layout"), channel2ChannelLayout(self.OutChannels), 0);
	C.av_opt_set_int(unsafe.Pointer(swr_ctx), C.CString("out_sample_rate"), C.int64_t(self.InSampleRate), 0);
	C.av_opt_set_sample_fmt(unsafe.Pointer(swr_ctx), C.CString("out_sample_fmt"), C.int64_t(C.AV_SAMPLE_FMT_S16), 0);

	C.swr_init(swr_ctx)

	self.resample = swr_ctx

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

func (self *ATranscorder) Do(in []byte) (out []byte, err error) {

	return nil, nil
}

func (self *ATranscorder) Close() {

}

func channel2ChannelLayout(channel int) (layout C.int64_t) {

	if channel == 1 {
		return C.int64_t(C.AV_CH_LAYOUT_MONO)
	}
	if channel == 2 {
		return C.int64_t(C.AV_CH_LAYOUT_STEREO)
	}
	return 0
}
