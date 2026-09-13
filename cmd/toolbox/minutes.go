package main

import (
	"github.com/spf13/cobra"
)

func newMinutesCommand() *cobra.Command {
	var (
		url         string
		sourceLang  string
		targetLang  string
		features    string
		speakers    int
		hotwords    string
		allActivate bool
		wordTS      bool
		outDir      string
		jsonOut     bool
	)
	cmd := &cobra.Command{
		Use:   "minutes <url>",
		Short: "语音妙记：音视频 URL 转结构化纪要（转写+说话人+总结+待办+章节+翻译）",
		Long: `语音妙记（lark minutes）：提交公网可访问的音视频 URL，异步生成结构化纪要。
转写必产（带说话人，附 txt 全文与 SRT 字幕）；附加功能至少一项：
summary 全文总结 / todo 待办 / qa 问答 / chapter 章节 / translation 翻译（中英互转）。
限制：文件 <1G、时长 ≤2 小时；本地文件请先上传到可访问的存储。
生成耗时与音视频时长正相关（分钟级），建议后台执行并轮询进程退出。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 1 {
				url = args[0]
			}
			params := map[string]any{"url": url, "features": features}
			if c.Flags().Changed("lang") {
				params["source_lang"] = sourceLang
			}
			if c.Flags().Changed("target-lang") {
				params["target_lang"] = targetLang
			}
			if c.Flags().Changed("speakers") {
				params["speakers"] = speakers
			}
			if hotwords != "" {
				params["hotwords"] = hotwords
			}
			if c.Flags().Changed("all-activate") {
				params["all_activate"] = allActivate
			}
			if wordTS {
				params["word_timestamps"] = true
			}
			if outDir != "" {
				params["_out"] = outDir
			}
			return runToolSync(c, "volcengine", "minutes", params, nil, "", jsonOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&sourceLang, "lang", "zh_cn", "源语种: zh_cn|en_us")
	f.StringVar(&targetLang, "target-lang", "en_us", "翻译目标语（features 含 translation 时生效）")
	f.StringVar(&features, "features", "summary", "附加功能（逗号分隔，至少一项）: summary|todo|qa|chapter|translation")
	f.IntVar(&speakers, "speakers", 0, "说话人数（0=自动识别）")
	f.StringVar(&hotwords, "hotwords", "", "热词，逗号分隔")
	f.BoolVar(&allActivate, "all-activate", true, "打包计费（false 按所选功能汇总计费）")
	f.BoolVar(&wordTS, "word-timestamps", false, "需要字级时间序列")
	f.StringVar(&outDir, "out-dir", "", "结果输出目录（默认数据目录自动命名）")
	f.BoolVar(&jsonOut, "json", false, "stdout 输出机器可读 JSON")
	return cmd
}
