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

package txrxmuxer

import (
	"go-rtsp-yolo8-tortmp/yolo8"
	"image"

	"gocv.io/x/gocv"
)

type ImageProcessor struct {
	In   chan image.Image
	Out  chan gocv.Mat
	quit chan struct{}
	YM   *yolo8.Yolo8Model
}

func NewProcessor(YM *yolo8.Yolo8Model) *ImageProcessor {
	return &ImageProcessor{
		In:   make(chan image.Image, 1000),
		Out:  make(chan gocv.Mat, 1000),
		quit: make(chan struct{}),
		YM:   YM,
	}
}

func (p *ImageProcessor) Start() {
	go func() {
		for {
			select {
			case msg := <-p.In:
				// fmt.Println("ImageProcessor Receive Data:", msg.Bounds())
				mat, ok := p.processMessage(msg)
				if ok {
					p.Out <- mat
				}
			case <-p.quit:
				return
			}
		}
	}()
}

// Stop 停止处理器
func (p *ImageProcessor) Stop() {
	p.quit <- struct{}{}
}

func (p *ImageProcessor) processMessage(img image.Image) (gocv.Mat, bool) {
	return p.YM.Yolo8Handler(img)
}
