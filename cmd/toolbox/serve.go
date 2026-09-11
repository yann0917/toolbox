package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/yann0917/toolbox/internal/config"
	"github.com/yann0917/toolbox/internal/server"
	"github.com/yann0917/toolbox/internal/service"
)

//go:embed all:webdist
var webDist embed.FS

func newServeCommand() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 Web 控制台",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if port != 0 {
				cfg.Server.Port = port
			}
			svc, err := service.New(cfg)
			if err != nil {
				return err
			}
			defer svc.Close()
			srv := server.New(svc)
			svc.StartEngine(srv.Hub().Notify, 2)

			handler := server.WithStatic(srv.Handler(), webDist)
			addr := fmt.Sprintf("127.0.0.1:%d", cfg.Server.Port)
			fmt.Fprintf(os.Stderr, "toolbox Web 已启动: http://%s\n", addr)
			return http.ListenAndServe(addr, handler)
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "端口（默认取配置）")
	return cmd
}
