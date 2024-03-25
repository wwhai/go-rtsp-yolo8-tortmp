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
	"sync"
	"time"

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
var __Cgo_Locker = sync.Mutex{}

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
	// SyncMatChannel := make(chan gocv.Mat, 1)
	// go func() {
	// 	NewestFrameMat := gocv.NewMat()
	// 	defer func() {
	// 		fmt.Println("推流进程退出")
	// 	}()
	// 	for {
	// 		select {
	// 		case <-context.Background().Done():
	// 			return
	// 		case <-SyncMatChannel:
	// 		default:

	// 		}
	// 		__Cgo_Locker.Lock()
	// 		Size := NewestFrameMat.Size()
	// 		__Cgo_Locker.Unlock()
	// 		if len(Size) < 2 {
	// 			fmt.Println("NewestFrameMat.Size() <2")
	// 			continue
	// 		}
	// 		__Cgo_Locker.Lock()
	// 		pngData, err3 := gocv.IMEncode(gocv.PNGFileExt, NewestFrameMat)
	// 		__Cgo_Locker.Unlock()
	// 		if err3 != nil {
	// 			log.Println("IMEncode ", err3)
	// 			pngData.Close()
	// 			return
	// 		}
	// 		__Cgo_Locker.Lock()
	// 		NewestFrameMat.Close()
	// 		__Cgo_Locker.Unlock()
	// 		if err := FFMpegProcess.WritePNG(pngData.GetBytes()); err != nil {
	// 			fmt.Println("WritePNG error", err)
	// 			pngData.Close()
	// 			return
	// 		}
	// 		pngData.Close()
	// 	}
	// }()
	c.OnPacketRTP(media, forma, func(pkt *rtp.Packet) {
		time.Sleep(10 * time.Millisecond)
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
		defer func() {
			nalus = nil
		}()

		for _, nalu := range nalus {
			// convert NALUs into RGBA frames
			img, err1 := frameDec.Decode(nalu)
			if err1 != nil {
				panic(err1)
			}
			if img == nil {
				continue
			}
			pngDataNativeByteBuffer, ok := ImageMatToPngByte(img)
			if !ok {
				continue
			}
			defer pngDataNativeByteBuffer.Close()
			if err4 := FFMpegProcess.WritePNG(pngDataNativeByteBuffer.GetBytes()); err4 != nil {
				fmt.Println("WritePNG error", err4)
				continue
			}
			// OverlappedMat, ok := Yolo8Handler(Yolo8Net, pkt, img)
			// defer OverlappedMat.Close()
			// if !ok {
			// 	continue
			// }
			// RGBFormatMat, err2 := gocv.ImageToMatRGB(img)
			// if err2 != nil {
			// 	log.Println("ImageToMatRGB error:", err2)
			// 	continue
			// }
			// defer RGBFormatMat.Close()
			// pngData, err3 := gocv.IMEncode(gocv.PNGFileExt, RGBFormatMat)
			// if err3 != nil {
			// 	log.Println("IMEncode error:", err3)
			// 	continue
			// }
			// defer pngData.Close()
			// if err2 := FFMpegProcess.WritePNG(pngData.GetBytes()); err2 != nil {
			// 	fmt.Println("WritePNG error", err2)
			// 	continue
			// }
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

/*
*
* 直接吧Mat转成PNG流
*
 */
func ImageMatToPngByte(img image.Image) (*gocv.NativeByteBuffer, bool) {
	__Cgo_Locker.Lock()
	defer __Cgo_Locker.Unlock()
	RGBFormatMat, err2 := gocv.ImageToMatRGB(img)
	if err2 != nil {
		log.Println("ImageToMatRGB error:", err2)
		return nil, false
	}
	defer RGBFormatMat.Close()
	pngDataNativeByteBuffer, err3 := gocv.IMEncode(gocv.PNGFileExt, RGBFormatMat)
	if err3 != nil {
		log.Println("IMEncode error:", err3)
		return pngDataNativeByteBuffer, false
	}
	return pngDataNativeByteBuffer, true
}

/*
*
* Yolo8融合,返回 Yolo8RGBFormatMat
*
 */
func Yolo8Handler(Yolo8Net gocv.Net, pkt *rtp.Packet, img image.Image) (gocv.Mat, bool) {
	__Cgo_Locker.Lock()
	defer __Cgo_Locker.Unlock()
	zoomedImgMat := gocv.NewMat()
	defer zoomedImgMat.Close()
	var err2 error
	// Yolo8: input -> 1*3*84
	Yolo8RGBFormatMat, err2 := gocv.ImageToMatRGB(img)
	if err2 != nil {
		log.Println("ImageToMatRGB ", err2)
		return Yolo8RGBFormatMat, false
	}
	gocv.Resize(Yolo8RGBFormatMat, &zoomedImgMat,
		image.Point{640, 640}, 0, 0, gocv.InterpolationLinear)
	blobMat := gocv.BlobFromImage(zoomedImgMat, 1/255.0,
		image.Point{640, 640}, gocv.Scalar{}, true, false)
	defer blobMat.Close()
	if blobMat.Ptr() == nil {
		return Yolo8RGBFormatMat, false
	}
	Yolo8Net.SetInput(blobMat, "")
	DnnOutput := Yolo8Net.Forward("")
	defer DnnOutput.Close()
	CalculateForwardResult := (CalculateForward(DnnOutput))
	for _, Result := range CalculateForwardResult {
		X := Result.Rectangle.Min.X * 3
		Y := int(math.Round(float64(Result.Rectangle.Min.Y) * (float64(1.685))))
		W := Result.Rectangle.Max.X * 3
		H := int(math.Round(float64(Result.Rectangle.Max.Y) * (float64(1.685))))
		gocv.Rectangle(&Yolo8RGBFormatMat, image.Rectangle{
			Min: image.Point{X: X, Y: Y},
			Max: image.Point{X: W, Y: H},
		}, color.RGBA{0, 255, 0, 0}, 2)
		fmt.Println("检测到目标:", Result.Class, ",", yolo8.CoCo8ClassesCN[Result.Class], pkt.SequenceNumber, pkt.Timestamp)
		gocv.PutText(&Yolo8RGBFormatMat, fmt.Sprintf("ClassId: %d, Score: %f",
			Result.Class, Result.Score), image.Point{X: X, Y: Y},
			gocv.FontHersheySimplex, 1, color.RGBA{255, 0, 0, 255}, 2)
	}
	return Yolo8RGBFormatMat, true
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
		confidenceValue := [80]float32{}
		for i := 4; i < cols; i++ {
			confidenceValue[i-4] = ptr[j+rows*i]
		}
		bestId, bestScore := getBestFromConfidenceValue(confidenceValue[:])
		scores[j] = bestScore
		boxes[j] = image.Rect(int(x-w/2), int(y-h/2), int(x+w/2), int(y+h/2))
		classIndexLists[j] = bestId
	}
	indices := gocv.NMSBoxes(boxes[:], scores[:], 0.25, 0.5)
	for _, indic := range indices {
		Box := boxes[indic]
		Score := scores[indic]
		Class := classIndexLists[indic]
		OutResult = append(OutResult, Result{
			Rectangle: Box,
			Score:     Score,
			Class:     Class,
		})
	}
	return OutResult
}
func getBestFromConfidenceValue(confidenceValues []float32) (int, float32) {
	bestId := 0
	bestScore := float32(0)
	for i, confidenceValue := range confidenceValues {
		if confidenceValue > bestScore {
			bestId = i
			bestScore = confidenceValue
		}
	}
	return bestId, bestScore
}
