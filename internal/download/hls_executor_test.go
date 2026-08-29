package download

import (
	"res-downloader/internal/config"
)

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	shared "res-downloader/internal/model"
	"testing"
)

func TestParseHLSVariantsAndSelection(t *testing.T) {
	playlist := []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=100,RESOLUTION=640x360\nlow/index.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=300,RESOLUTION=1920x1080\nhigh/index.m3u8\n")
	variants, err := parseHLSVariants(playlist, "https://cdn.example/master.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) != 2 {
		t.Fatalf("variants = %d", len(variants))
	}
	if got := selectHLSVariant(variants, nil).url; got != "https://cdn.example/high/index.m3u8" {
		t.Fatalf("highest variant = %q", got)
	}
	if got := selectHLSVariant(variants, map[string]interface{}{"variant": "lowest"}).url; got != "https://cdn.example/low/index.m3u8" {
		t.Fatalf("lowest variant = %q", got)
	}
}

func TestParseHLSMediaPlaylist(t *testing.T) {
	playlist := []byte("#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:7\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\",IV=0x01\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:1,\nsegment.m4s\n#EXT-X-BYTERANGE:4@8\nfile.m4s\n#EXT-X-ENDLIST\n")
	parts, endList, err := parseHLSMediaPlaylist(playlist, "https://cdn.example/path/list.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if !endList || len(parts) != 3 {
		t.Fatalf("endList=%v parts=%d", endList, len(parts))
	}
	if parts[0].url != "https://cdn.example/path/init.mp4" || parts[1].sequence != 7 {
		t.Fatalf("parts = %#v", parts)
	}
	if parts[2].rangeStart != 8 || parts[2].rangeEnd != 11 {
		t.Fatalf("byte range = %d-%d", parts[2].rangeStart, parts[2].rangeEnd)
	}
}

func TestDecryptHLSSegment(t *testing.T) {
	key := []byte("0123456789abcdef")
	plain := []byte("hello hls")
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(append([]byte(nil), plain...), bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	iv, _ := hex.DecodeString("00000000000000000000000000000007")
	encrypted := append([]byte(nil), padded...)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, encrypted)
	decrypted, err := decryptHLSSegment(encrypted, key, nil, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestHLSDownloadPlanRunner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/master.m3u8":
			_, _ = writer.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=10\nmedia.m3u8\n"))
		case "/media.m3u8":
			_, _ = writer.Write([]byte("#EXTM3U\n#EXTINF:1,\none.ts\n#EXTINF:1,\ntwo.ts\n#EXT-X-ENDLIST\n"))
		case "/one.ts":
			_, _ = writer.Write([]byte("one"))
		case "/two.ts":
			_, _ = writer.Write([]byte("two"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	runner := NewPlanRunner(nil, &config.Config{UserAgent: "res-downloader-test"}, nil, nil, nil)
	path, err := runner.Run(shared.DownloadPlan{
		Inputs: []shared.DownloadInput{{ID: "hls", Executor: "hls", URL: server.URL + "/master.m3u8"}},
		Output: shared.DownloadOutput{Input: "hls", Extension: ".ts"},
	}, t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "onetwo" {
		t.Fatalf("output = %q", data)
	}
}
