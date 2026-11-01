package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	telegramsvc "github.com/fr0der1c/tgtldr/app/internal/telegram"
)

func main() {
	output := flag.String("output", "", "local export directory (required)")
	flag.Parse()
	if *output == "" {
		fmt.Fprintln(os.Stderr, "请指定本机导出目录。")
		os.Exit(2)
	}
	if !config.StorageProtected() {
		fmt.Fprintln(os.Stderr, "请通过已验证的加密存储环境启动导出。")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "无法加载本机配置。")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	st, err := store.Open(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "无法连接本机数据库。")
		os.Exit(1)
	}
	defer st.Close()
	rows, err := st.Pool.Query(ctx, `select a.id from telegram_accounts a where a.status='authorized' and (a.telegram_user_id=(select owner_id from activity_config where id=1) or (select count(*) from telegram_accounts where status='authorized')=1)`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "无法读取已授权账号状态。")
		os.Exit(1)
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			fmt.Fprintln(os.Stderr, "账号状态读取失败。")
			os.Exit(1)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(ids) != 1 {
		fmt.Fprintln(os.Stderr, "需要一个与私人助手绑定的已授权账号。")
		os.Exit(1)
	}
	loc, _ := time.LoadLocation("Asia/Singapore")
	end := time.Now().In(loc)
	start := end.AddDate(0, 0, -30)
	fmt.Println("开始导出最近30天的群组和频道文字；不读取私聊历史，不发送消息，不调用AI。")
	service := telegramsvc.NewService(ctx, st, clock.System{}, cfg.MediaDir)
	err = service.ExportText(ctx, ids[0], *output, start, end, func(index, total, count int, status string) {
		fmt.Printf("groups=%d/%d current_messages=%d status=%s\n", index, total, count, status)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "导出中断；已完成的本机记录会保留，可对同一目录重试。错误类别：export_failed")
		os.Exit(1)
	}
	fmt.Println("导出任务结束；完整性以 manifest.json 为准。")
}
