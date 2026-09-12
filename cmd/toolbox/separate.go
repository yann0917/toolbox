package main

import (
	"github.com/spf13/cobra"
)

func newSeparateCommand() *cobra.Command {
	var (
		scene   string
		format  string
		outDir  string
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "separate <url>",
		Short: "人声背景音分离：公网音视频 URL 分离多轨音频",
		Long:  "从公网可访问的音视频 URL 分离人声与背景音（AI MediaKit）。\n请提供公网可访问的音视频 URL（MediaKit 不支持本地文件，本地文件请先上传到可访问的存储）。",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			params := map[string]any{"url": args[0], "scene": scene, "output_format": format}
			return runToolSync(c, "volcengine", "separate", params, nil, outDir, jsonOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&scene, "scene", "audio", "分离场景: audio|music|drama|narrate")
	f.StringVar(&format, "format", "mp3", "输出格式: aac|mp3|wav|m4a|flac")
	f.StringVar(&outDir, "out-dir", "", "音轨输出目录（默认数据目录自动命名）")
	f.BoolVar(&jsonOut, "json", false, "stdout 输出机器可读 JSON")
	return cmd
}
