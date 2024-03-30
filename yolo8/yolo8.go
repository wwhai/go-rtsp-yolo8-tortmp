package yolo8

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"sync"

	"gocv.io/x/gocv"
)

var __Cgo_Locker = sync.Mutex{}

type Yolo8Model struct {
	Yolo8Net gocv.Net
}

func NewYolo8Model() *Yolo8Model {
	return &Yolo8Model{
		Yolo8Net: gocv.Net{},
	}
}
func (M *Yolo8Model) LoadONNX() {
	log.Println("加载 yolov8n.onnx")
	M.Yolo8Net = gocv.ReadNetFromONNX("./yolov8n.onnx")
	M.Yolo8Net.SetPreferableBackend(gocv.NetBackendCUDA)
	M.Yolo8Net.SetPreferableTarget(gocv.NetTargetCUDA)
	log.Println("加载 yolov8n.onnx 完成")
}
func (M *Yolo8Model) UnLoadONNX() {
	M.Yolo8Net.Close()
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
* Yolo8融合,返回 Yolo8 RGB Format Mat
*
 */
func (M *Yolo8Model) Yolo8Handler(img image.Image) (gocv.Mat, bool) {
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
	M.Yolo8Net.SetInput(blobMat, "")
	DnnOutput := M.Yolo8Net.Forward("")
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
		fmt.Println("检测到目标:", Result.Class, ",", CoCo8ClassesCN[Result.Class])
		gocv.PutText(&Yolo8RGBFormatMat, fmt.Sprintf("ClassId: %d, Score: %f",
			Result.Class, Result.Score), image.Point{X: X, Y: Y},
			gocv.FontHersheySimplex, 1, color.RGBA{255, 0, 0, 255}, 2)
	}
	return Yolo8RGBFormatMat, true
}

type Result struct {
	Rectangle image.Rectangle
	Class     int
	ClassName string
	Score     float32
}

func (O Result) String() string {
	O.ClassName = CoCo8ClassesCN[O.Class]
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
