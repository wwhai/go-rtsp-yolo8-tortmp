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
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

type StreamPusher struct {
	Cmd   *exec.Cmd
	Stdin io.WriteCloser
}

// NewStreamPusher 创建一个新的推流器实例
func NewStreamPusher(rtmpURL string) (*StreamPusher, error) {
	// 构建 FFmpeg 推流命令
	cmd := exec.Command(
		"./ffmpeg.exe",
		"-f", "image2pipe",
		"-vcodec", "png",
		"-i", "-",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-preset", "fast",
		"-r", "25",
		"-f", "flv",
		rtmpURL,
	)

	// 获取 FFmpeg 的标准输入
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdin pipe: %v", err)
	}

	return &StreamPusher{
		Cmd:   cmd,
		Stdin: stdin,
	}, nil
}

func (p *StreamPusher) StartPush() error {
	if err := p.Cmd.Start(); err != nil {
		return fmt.Errorf("error starting FFmpeg command: %v", err)
	}
	return nil
}

func (p *StreamPusher) WritePNG(pngData []byte) error {
	if _, err := p.Stdin.Write(pngData); err != nil {
		return fmt.Errorf("error writing PNG data to stdin: %v", err)
	}
	return nil
}

func (p *StreamPusher) Close() {
	p.Stdin.Close()
	p.Cmd.Wait()
}

func main1() {
	rtmpURL := "rtmp://127.0.0.1:1935/live/testv001"
	pusher, err := NewStreamPusher(rtmpURL)
	if err != nil {
		fmt.Println(err)
		return
	}

	if err := pusher.StartPush(); err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		for {
			pngData, _ := os.ReadFile("./1originImgMat.png")
			if err := pusher.WritePNG(pngData); err != nil {
				fmt.Println(err)
				return
			}
			time.Sleep(time.Second / 25)
		}
	}()

	// 模拟推流一段时间后关闭
	time.Sleep(10 * time.Second)
	pusher.Close()
}
