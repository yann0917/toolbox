package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newServeCommand Web 控制台占位命令，Task 14 实现。
func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "启动 Web 控制台（即将推出）",
		RunE: func(c *cobra.Command, args []string) error {
			fmt.Println("serve 将在 Web 集成后可用")
			return nil
		},
	}
}
