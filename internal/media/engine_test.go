package media

import (
	"reflect"
	"testing"
)

func TestPluginFFmpegArgumentsRequireManagedInputsAndOutput(t *testing.T) {
	valid := []string{"-i", "{{input.0}}", "-c", "copy", "{{output}}"}
	if err := validatePluginFFmpegArguments(valid, 1); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-i", "/etc/passwd", "{{output}}"},
		{"-i", "https://example.com/live.m3u8", "{{output}}"},
		{"-filter_script", "filters.txt", "{{output}}"},
		{"-i", "{{input.0}}", "relative-output.mp4"},
	} {
		if err := validatePluginFFmpegArguments(args, 1); err == nil {
			t.Fatalf("unsafe arguments accepted: %#v", args)
		}
	}
}

func TestMediaMuxArgumentsSelectOneVideoAndAudioStream(t *testing.T) {
	want := []string{"-i", "video.mp4", "-i", "audio.webm", "-map", "0:v:0?", "-map", "1:a:0?", "-c", "copy", "output.mkv"}
	if got := mediaMuxArguments([]string{"video.mp4", "audio.webm"}, "output.mkv"); !reflect.DeepEqual(got, want) {
		t.Fatalf("mux arguments are %#v", got)
	}
}
