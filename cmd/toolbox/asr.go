package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newASRCommand() *cobra.Command {
	var (
		audioURL string
		outPath  string
		srt      bool
		hotwords string
		language string
		version  string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "asr <file>",
		Short: "语音识别：音频转文字（本地文件与公网 URL 二选一）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			file := ""
			if len(args) == 1 {
				file = args[0]
			}
			if file != "" && audioURL != "" {
				return fmt.Errorf("音频文件与 URL 只能提供其一")
			}
			if file == "" && audioURL == "" {
				return fmt.Errorf("请提供音频文件或 --url")
			}
			switch version {
			case "standard", "idle", "flash":
			default:
				return fmt.Errorf("识别版本 version 仅支持 standard / idle / flash")
			}

			var files map[string]string
			if file != "" {
				if _, err := os.Stat(file); err != nil {
					return fmt.Errorf("音频文件不存在: %s", file)
				}
				abs, err := filepath.Abs(file)
				if err != nil {
					return fmt.Errorf("解析音频文件路径失败: %w", err)
				}
				files = map[string]string{"audio": abs}
			}

			params := map[string]any{"hotwords": hotwords, "language": language, "srt": srt, "version": version}
			if audioURL != "" {
				params["url"] = audioURL
			}
			return runToolSync(c, "volcengine", "asr", params, files, outPath, jsonOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&audioURL, "url", "", "公网音频 URL（与位置参数二选一）")
	f.StringVar(&outPath, "out", "", "转写文本输出路径（默认数据目录自动命名）")
	f.BoolVar(&srt, "srt", true, "额外产出 SRT 字幕（--srt=false 关闭）")
	f.StringVar(&hotwords, "hotwords", "", "热词，逗号分隔（原样透传）")
	f.StringVar(&language, "language", "zh-CN", "识别语言（闲时/极速版留空自动识别）")
	f.StringVar(&version, "version", "standard", "识别版本：standard 标准版 / idle 闲时版（URL，24h 内完成） / flash 极速版（URL，秒级）")
	f.BoolVar(&jsonOut, "json", false, "stdout 输出机器可读 JSON")
	return cmd
}
