package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yann0917/toolbox/internal/server"
)

func newVoicesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "voices", Short: "音色查询"}
	list := &cobra.Command{
		Use:   "list",
		Short: "列出内置可用音色",
		RunE: func(c *cobra.Command, args []string) error {
			jsonOut, _ := c.Flags().GetBool("json")
			voices := server.BuiltinVoices()
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(voices)
			}
			for _, v := range voices {
				fmt.Printf("%-46s %-4s %s\n", v["id"], v["gender"], v["category"])
			}
			return nil
		},
	}
	list.Flags().Bool("json", false, "stdout 输出机器可读 JSON")
	cmd.AddCommand(list)
	return cmd
}
