package main

import (
	"os"
	"tv_streamer/helpers"
	"tv_streamer/helpers/logs"
	"tv_streamer/modules/streamer"
	"tv_streamer/modules/web"
)

func init() {
	if !helpers.IsFFmpegInstalled() {
		logs.GetLogger().Info(`ffmpeg is not installed`)
		os.Exit(1)
	}

	logs.GetLogger().Info(`Starting ...`)
	helpers.GetXORM()
}

func main() {

	// close properly
	defer helpers.GetXORM().Close()

	go func() {
		streamer.StartStream()
	}()

	web.Run()
}
