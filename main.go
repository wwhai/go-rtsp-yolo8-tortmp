package main

import (
	"context"
	"fmt"
	"go-rtsp-yolo8-tortmp/decoder"
	pusher "go-rtsp-yolo8-tortmp/puher"
	"go-rtsp-yolo8-tortmp/txrxmuxer"
	"go-rtsp-yolo8-tortmp/yolo8"
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
	"gocv.io/x/gocv"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's an H264 format
// 3. decode the H264 format into RGBA frames

// This example requires the FFmpeg libraries, that can be installed with this command:
// apt install -y libavformat-dev libswscale-dev gcc pkg-config

func main() {
	Yolo8Model := yolo8.NewYolo8Model()
	Yolo8Model.LoadONNX()
	ImageProcessor := txrxmuxer.NewProcessor(Yolo8Model)
	ImageProcessor.Start()
	defer Yolo8Model.UnLoadONNX()
	c := gortsplib.Client{}
	// u, err := base.ParseURL("rtsp://admin:mju2024315@192.168.1.64:554/Streaming/Channels/1")
	u, err := base.ParseURL("rtsp://192.168.1.210:554/av0_0")
	if err != nil {
		panic(err)
	}
	FFMpegProcess, err4 := pusher.NewStreamPusher("rtmp://127.0.0.1:1935/live/testv001")
	if err4 != nil {
		panic(err4)
	}
	go FFMpegProcess.StartPush()
	defer FFMpegProcess.Close()
	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	desc, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	var forma *format.H264
	media := desc.FindFormat(&forma)
	if media == nil {
		panic("media not found")
	}

	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	frameDec := &decoder.H264Decoder{}
	err = frameDec.Initialize()
	if err != nil {
		panic(err)
	}
	defer frameDec.Close()

	if forma.SPS != nil {
		frameDec.Decode(forma.SPS)
	}
	if forma.PPS != nil {
		frameDec.Decode(forma.PPS)
	}

	_, err = c.Setup(desc.BaseURL, media, 0, 0)
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			select {
			case <-context.Background().Done():
				return
			case OverlappedRgbMat := <-ImageProcessor.Out:
				pngData, err3 := gocv.IMEncode(gocv.PNGFileExt, OverlappedRgbMat)
				if err3 != nil {
					log.Println("IMEncode error:", err3)
					OverlappedRgbMat.Close()
					pngData.Close()
					continue
				}
				if err2 := FFMpegProcess.WritePNG(pngData.GetBytes()); err2 != nil {
					fmt.Println("WritePNG error", err2)
					OverlappedRgbMat.Close()
					continue
				}
				pngData.Close()
				OverlappedRgbMat.Close()
			}
		}
	}()
	c.OnPacketRTP(media, forma, func(pkt *rtp.Packet) {
		_, ok := c.PacketPTS(media, pkt)
		// log.Printf("PTS %v and Timestamp %v", pts, pkt.Timestamp)
		if !ok {
			log.Printf("waiting for timestamp")
			return
		}
		nalus, err := rtpDec.Decode(pkt)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, nalu := range nalus {
			// convert NALUs into RGBA frames
			img, err1 := frameDec.Decode(nalu)
			if err1 != nil {
				panic(err1)
			}
			if img == nil {
				fmt.Println("{img==nil}")
				continue
			}
			ImageProcessor.In <- img
			pngDataNativeByteBuffer, ok := yolo8.ImageMatToPngByte(img)
			if !ok {
				pngDataNativeByteBuffer.Close()
				continue
			}
			if err4 := FFMpegProcess.WritePNG(pngDataNativeByteBuffer.GetBytes()); err4 != nil {
				pngDataNativeByteBuffer.Close()
				fmt.Println("WritePNG error", err4)
				continue
			}
			pngDataNativeByteBuffer.Close()

		}
	})
	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
