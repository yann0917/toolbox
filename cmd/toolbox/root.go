package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "toolbox",
		Short:   "多媒体 AI 工具箱",
		Version: version,
	}
	root.AddCommand(
		newConfigCmd(),
		newVoicesCmd(),
		newTTSCommand(),
		newTTSLongCommand(),
		newTTSStreamCommand(),
		newASRCommand(),
		newPodcastCommand(),
		newSeparateCommand(),
		newTranslateCommand(),
		newServeCommand(),
	)
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(2)
	}
}
