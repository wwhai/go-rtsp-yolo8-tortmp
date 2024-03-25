// Copyright (C) 2024 wwhai
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"go-rtsp-yolo8-tortmp/decoder"
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's an H264 format
// 3. decode the H264 format into RGBA frames

// This example requires the FFmpeg libraries, that can be installed with this command:
// apt install -y libavformat-dev libswscale-dev gcc pkg-config

func main2() {
	c := gortsplib.Client{}
	// parse URL
	// u, err := base.ParseURL("rtsp://admin:mju2024315@192.168.1.64:554/Streaming/Channels/1")
	u, err := base.ParseURL("rtsp://192.168.1.210:554/av0_0")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	// find available medias，得到该地址流媒体的信息
	desc, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H264 media and format，找到符合要求的描述信息
	var forma *format.H264
	media := desc.FindFormat(&forma)
	if media == nil {
		panic("media not found")
	}

	// setup RTP/H264 -> H264 decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup H264 -> raw frames decoder
	frameDec := &decoder.H264Decoder{}
	err = frameDec.Initialize()
	if err != nil {
		panic(err)
	}
	defer frameDec.Close()

	// if SPS and PPS are present into the SDP, send them to the decoder
	if forma.SPS != nil {
		frameDec.Decode(forma.SPS)
	}
	if forma.PPS != nil {
		frameDec.Decode(forma.PPS)
	}

	// setup a single media,与拉流的地址建立联系
	_, err = c.Setup(desc.BaseURL, media, 0, 0)
	if err != nil {
		panic(err)
	}
	// called when a RTP packet arrives
	c.OnPacketRTP(media, forma, func(pkt *rtp.Packet) {
		// decode timestamp
		pts, ok := c.PacketPTS(media, pkt)
		if !ok {
			log.Printf("waiting for timestamp")
			return
		}

		// extract access units from RTP packets
		au, err := rtpDec.Decode(pkt)
		if err != nil {
			if err != rtph264.ErrNonStartingPacketAndNoPrevious && err != rtph264.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
			return
		}

		for _, nalu := range au {
			// convert NALUs into RGBA frames
			img, err := frameDec.Decode(nalu)
			if err != nil {
				panic(err)
			}

			// wait for a frame
			if img == nil {
				continue
			}

			log.Printf("decoded frame with PTS %v and size %v", pts, img.Bounds().Max)
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
