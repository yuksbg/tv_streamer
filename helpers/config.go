package helpers

import (
	"strings"
	"sync"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type myConfig2 struct {
	App struct {
		WebPort        int    `yaml:"web_port" koanf:"web_port"`
		VideoFilesPath string `yaml:"video_files_path" koanf:"video_files_path"`
	} `yaml:"app" koanf:"app"`
	Database struct {
		DBPath string `yaml:"db_path" koanf:"db_path"`
	} `yaml:"database" koanf:"database"`
	Streaming struct {
		OutputDir      string `yaml:"output_dir" koanf:"output_dir"`
		HlsSegmentTime int    `yaml:"hls_segment_time" koanf:"hls_segment_time"`
		HlsListSize    int    `yaml:"hls_list_size" koanf:"hls_list_size"`
		FFmpegPreset   string `yaml:"ffmpeg_preset" koanf:"ffmpeg_preset"`
		VideoBitrate   string `yaml:"video_bitrate" koanf:"video_bitrate"`
		AudioBitrate   string `yaml:"audio_bitrate" koanf:"audio_bitrate"`
	} `yaml:"streaming" koanf:"streaming"`
}

var loadedConfig *myConfig2
var loadedConfigOnce sync.Once

func GetConfig() *myConfig2 {
	loadedConfigOnce.Do(func() {
		var k = koanf.New(".")
		if err := k.Load(file.Provider("config.yaml"), yaml.Parser()); err != nil {
			panic(err.Error())
		}
		if err := k.Load(env.Provider("APP_", ".", func(s string) string {
			return strings.Replace(strings.ToLower(
				strings.TrimPrefix(s, "APP_")), "_", ".", -1)
		}), nil); err != nil {
			panic(err.Error())
		}
		if err := k.Unmarshal("", &loadedConfig); err != nil {
			panic(err.Error())
		}
	})
	return loadedConfig
}
