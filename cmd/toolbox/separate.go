package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSeparateCommand() *cobra.Command {
	var (
		scene   string
		format  string
		outDir  string
		jsonOut bool
		file    string
	)
	cmd := &cobra.Command{
		Use:   "separate <url>",
		Short: "人声背景音分离：公网音视频 URL 或本地文件分离多轨音频",
		Long:  "从公网可访问的音视频 URL（或 --file 本地文件，需已配置对象存储中转）分离人声与背景音（AI MediaKit）。",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			var url string
			if len(args) == 1 {
				url = args[0]
			}
			if file != "" && url != "" {
				return fmt.Errorf("--file 与 URL 参数只能提供其一")
			}
			var files map[string]string
			if file != "" {
				// 本地文件输入：任务执行期经对象存储中转为签名 URL（未配置存储时任务报设置指引）
				files = map[string]string{"audio": file}
			}
			params := map[string]any{"url": url, "scene": scene, "output_format": format}
			return runToolSync(c, "volcengine", "separate", params, files, outDir, jsonOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&scene, "scene", "audio", "分离场景: audio|music|drama|narrate")
	f.StringVar(&format, "format", "mp3", "输出格式: aac|mp3|wav|m4a|flac")
	f.StringVar(&outDir, "out-dir", "", "音轨输出目录（默认数据目录自动命名）")
	f.BoolVar(&jsonOut, "json", false, "stdout 输出机器可读 JSON")
	f.StringVar(&file, "file", "", "本地音视频文件路径（配置对象存储后自动中转，与 URL 参数互斥）")
	return cmd
}
