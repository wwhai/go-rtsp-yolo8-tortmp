package main

import (
	"encoding/json"
	"fmt"
	"go-rtsp-yolo8-tortmp/decoder"
	pusher "go-rtsp-yolo8-tortmp/puher"
	"go-rtsp-yolo8-tortmp/yolo8"
	"image"
	"image/color"
	"log"
	"math"

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
	log.Println("加载 yolov8n.onnx")
	Yolo8Net := gocv.ReadNetFromONNX("./yolov8n.onnx")
	Yolo8Net.SetPreferableBackend(gocv.NetBackendCUDA)
	Yolo8Net.SetPreferableTarget(gocv.NetTargetCUDA)
	log.Println("加载 yolov8n.onnx 完成")
	FFMpegProcess, err4 := pusher.NewStreamPusher("rtmp://127.0.0.1:1935/live/testv001")
	if err4 != nil {
		panic(err4)
	}
	go FFMpegProcess.StartPush()

	c.OnPacketRTP(media, forma, func(pkt *rtp.Packet) {
		// decode timestamp
		_, ok := c.PacketPTS(media, pkt)
		// log.Printf("PTS %v and Timestamp %v", pts, pkt.Timestamp)

		if !ok {
			log.Printf("waiting for timestamp")
			return
		}

		nalus, err := rtpDec.Decode(pkt)
		if err != nil {
			return
		}

		for _, nalu := range nalus {
			// convert NALUs into RGBA frames
			img, err := frameDec.Decode(nalu)
			if err != nil {
				panic(err)
			}

			// wait for a frame
			if img == nil {
				continue
			}

			NewestFrameMat, err2 := gocv.ImageToMatRGB(img)
			if err2 != nil {
				log.Println("ImageToMatRGB ", err2)
				continue
			}
			defer NewestFrameMat.Close()
			zoomedImgMat := gocv.NewMat()
			defer zoomedImgMat.Close()
			gocv.Resize(NewestFrameMat, &zoomedImgMat,
				image.Point{640, 640}, 0, 0, gocv.InterpolationLinear)
			blobMat := gocv.BlobFromImage(zoomedImgMat, 1/255.0,
				image.Point{640, 640}, gocv.Scalar{}, true, false)
			defer blobMat.Close()
			if blobMat.Ptr() == nil {
				continue
			}

			Yolo8Net.SetInput(blobMat, "")
			DnnOutput := Yolo8Net.Forward("")
			defer DnnOutput.Close()
			CalculateForwardResult := (CalculateForward(DnnOutput))
			for _, Result := range CalculateForwardResult {
				// log.Println(Result)
				X := Result.Rectangle.Min.X * 3
				Y := int(math.Round(float64(Result.Rectangle.Min.Y) * (float64(1.68))))
				W := Result.Rectangle.Max.X * 3
				H := int(math.Round(float64(Result.Rectangle.Max.Y) * (float64(1.68))))
				gocv.Rectangle(&NewestFrameMat, image.Rectangle{
					Min: image.Point{X: X, Y: Y},
					Max: image.Point{X: W, Y: H},
				}, color.RGBA{0, 255, 0, 0}, 2)
				fmt.Println("检测到目标:", Result.Class, ",", yolo8.CoCo8ClassesCN[Result.Class], pkt.Timestamp)
				gocv.PutText(&NewestFrameMat, fmt.Sprintf("ClassId: %d, Score: %f",
					Result.Class, Result.Score),
					image.Point{X: X, Y: Y},
					gocv.FontHersheySimplex, 1, color.RGBA{255, 0, 0, 255}, 2)
				pngData, err3 := gocv.IMEncode(gocv.PNGFileExt, NewestFrameMat)
				if err3 != nil {
					log.Println("IMEncode ", err3)
					return
				}
				defer pngData.Close()
				if err := FFMpegProcess.WritePNG(pngData.GetBytes()); err != nil {
					fmt.Println("WritePNG ", err)
					return
				}

			}
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

type Result struct {
	Rectangle image.Rectangle
	Class     int
	Score     float32
}

func (O Result) String() string {
	if bytes, err := json.Marshal(O); err != nil {
		return "{}"
	} else {
		return string(bytes)
	}
}

/*
*
* 计算返回值
*
 */
func CalculateForward(outs gocv.Mat) []Result {
	OutResult := []Result{}
	cols := 84
	rows := 8400
	ptr, _ := outs.DataPtrFloat32()
	boxes := [8400]image.Rectangle{}
	scores := [8400]float32{}
	classIndexLists := [8400]int{}
	for j := 0; j < rows; j++ {
		x := ptr[j]
		y := ptr[j+rows]
		w := ptr[j+rows*2]
		h := ptr[j+rows*3]
		confs := [80]float32{}
		for i := 4; i < cols; i++ {
			confs[i-4] = ptr[j+rows*i]
		}
		bestId, bestScore := getBestFromConfs(confs[:])
		scores[j] = bestScore
		boxes[j] = image.Rect(int(x-w/2), int(y-h/2), int(x+w/2), int(y+h/2))
		classIndexLists[j] = bestId
	}
	indices := gocv.NMSBoxes(boxes[:], scores[:], 0.25, 0.5)
	for _, indice := range indices {
		Box := boxes[indice]
		Score := scores[indice]
		Class := classIndexLists[indice]
		OutResult = append(OutResult, Result{
			Rectangle: Box,
			Score:     Score,
			Class:     Class,
		})
	}
	return OutResult
}
func getBestFromConfs(confs []float32) (int, float32) {
	bestId := 0
	bestScore := float32(0)
	for i, v := range confs {
		if v > bestScore {
			bestId = i
			bestScore = v
		}
	}
	return bestId, bestScore
}
