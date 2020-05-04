package main

import (
	"fmt"
	"github.com/notedit/acodec"
	"github.com/notedit/rtmp-lib"
	"github.com/notedit/rtmp-lib/av"
)

func main() {

	config := &rtmp.Config{
		ChunkSize:1024,
		BufferSize: 1024,
	}
	server := rtmp.NewServer(config)

	server.HandlePublish = func(conn *rtmp.Conn) {
		//
		//file, err := os.Create("test.opus")
		//if err != nil {
		//	panic(err)
		//}

		streams, err := conn.Streams()

		if err != nil {
			fmt.Println(err)
		}

		transcoder := &acodec.ATranscorder{}

		var adecodec av.AudioCodecData

		for _, stream := range streams {
			if stream.Type().IsAudio() {
				adecodec = stream.(av.AudioCodecData)

				fmt.Println(adecodec.SampleFormat().String())
				transcoder.DecodeCodecName = "aac"
				transcoder.InSampleRate = adecodec.SampleRate()
				transcoder.InChannels = adecodec.ChannelLayout().Count()

				transcoder.EncodeCodecName = "libopus"
				transcoder.OutBitrate = 96000
				transcoder.OutChannels = 2
				transcoder.OutSampleRate = 48000

				err := transcoder.Setup()

				if err != nil {
					panic(err)
				}
			}
		}

		for {
			packet, err := conn.ReadPacket()
			if err != nil {
				break
			}

			stream := streams[packet.Idx]

			if stream.Type().IsVideo() {
				continue
			}

			_, _, err = transcoder.Do(packet.Data)

			if err != nil {
				fmt.Println(err)
			}

			fmt.Println(packet.Time)
		}
	}

	server.ListenAndServe()

}
