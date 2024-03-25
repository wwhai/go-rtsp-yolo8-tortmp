package yolo8

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"time"

	"gocv.io/x/gocv"
)

var (
	modelFile      = flag.String("model", "yolov8n.onnx", "model path")
	modelImageSize = flag.Int("size", 640, "model image size")
	srcImage       = flag.String("image", "images/bus.jpg", "input image")
)

func __main() {
	flag.Parse()
	net := gocv.ReadNetFromONNX(*modelFile)
	net.SetPreferableBackend(gocv.NetBackendCUDA)
	net.SetPreferableTarget(gocv.NetTargetCUDA)
	src := gocv.IMRead(*srcImage, gocv.IMReadColor)
	modelSize := image.Pt(*modelImageSize, *modelImageSize)
	resized := gocv.NewMat()

	letterBox(src, &resized, modelSize)
	blob := gocv.BlobFromImage(resized, 1/255.0, modelSize, gocv.Scalar{}, true, false)
	var outs gocv.Mat

	net.SetInput(blob, "")
	outs = net.Forward("")

	start := time.Now()
	// sz := outs.Size() //outs形状是1*84*8400
	cols := 84   //84列，前4个是cx，cy，w，h，后80是80类的概率
	rows := 8400 //因为后后面要行转列所以这样命名 8400行，表示8400框

	// log.Println(sz)

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
	elapsed := time.Since(start)
	fmt.Println("该函数执行完成耗时：", elapsed)
	indices := gocv.NMSBoxes(boxes[:], scores[:], 0.25, 0.5)

	log.Println(indices)

	output := resized.Clone()
	for _, v := range indices {
		log.Println(v)

		gocv.Rectangle(&output, boxes[v], color.RGBA{0, 255, 0, 0}, 2)
		gocv.PutText(&output, fmt.Sprintf("ClassId: %d, Score: %f", classIndexLists[v], scores[v]),
			boxes[v].Min, gocv.FontHersheySimplex, 0.5, color.RGBA{255, 0, 0, 255}, 2)

	}

	w := gocv.NewWindow("detected")
	w.ResizeWindow(modelSize.X, modelSize.Y)
	w.IMShow(output)
	w.WaitKey(-1)

	src.Close()
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

func letterBox(src gocv.Mat, dst *gocv.Mat, size image.Point) {
	k := math.Min(float64(size.X)/float64(src.Cols()), float64(size.Y)/float64(src.Rows()))
	newSize := image.Pt(int(k*float64(src.Cols())), int(k*float64(src.Rows())))

	tmp := gocv.NewMat()
	gocv.Resize(src, &tmp, newSize, 0, 0, gocv.InterpolationLinear)

	if dst.Cols() != size.X || dst.Rows() != size.Y {
		dstNew := gocv.NewMatWithSize(size.Y, size.X, src.Type())
		dstNew.CopyTo(dst)
	}

	rectOfTmp := image.Rect(0, 0, newSize.X, newSize.Y)

	regionOfDst := dst.Region(rectOfTmp)
	tmp.CopyTo(&regionOfDst)
}
