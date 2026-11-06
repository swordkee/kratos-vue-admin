package media

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // 注册解码器（源 DecodeImageWidthHeight 支持 gif/jpeg/png/webp）
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"
)

// decodeResolution 读图片头取宽高，返回 "WxH"（源 upload.DecodeImageWidthHeight 同格式）。
//
// 只解码头部（image.DecodeConfig），不整图解码；非图片/损坏/不支持的格式一律返回 ""
// （源端解不出时同样留空，不阻断上传）。
func decodeResolution(r io.Reader) string {
	cfg, _, err := image.DecodeConfig(r)
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", cfg.Width, cfg.Height)
}

// bytesResolution 便捷入口（断点续传合并后探测用，小文件才用）。
func bytesResolution(b []byte) string {
	return decodeResolution(bytes.NewReader(b))
}
