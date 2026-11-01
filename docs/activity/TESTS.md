# 测试说明

[简体中文](TESTS.md) | [English](../TESTING.en.md)

## 在本机运行

后端集成测试需要两个独立、可删除的 PostgreSQL 数据库。测试会清理数据，不要连接实际使用的数据库。

```bash
cd app
ACTIVITY_TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/activity_test?sslmode=disable' \
TGTLDR_TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/upstream_test?sslmode=disable' \
go test -p 1 -count=1 ./...
```

未设置测试库时，数据库测试会跳过。模型和 Bot 测试使用本地模拟服务，不需要真实凭据。

网页：

```bash
cd web
node --test lib/calendar.test.mjs
npm run build
```

Windows 脚本：`powershell -NoProfile -File scripts/windows-tests.ps1`。分享文件检查：`python3 scripts/check-shareable.py`。这些检查都手动运行，仓库没有 GitHub 自动测试配置。

## 检查内容

- 英文日期、相对日期、跨日活动，以及缺少时间时不猜测。
- 活动转发、编辑、去重、时间冲突、重复按钮和状态保存。
- 同一房间的不同活动分别保存，不靠地点判断重复。
- 同一活动相同卡片不重发、真实修改与恢复旧内容仍通知、并发发送、回执保存、失败隔离及重启重试。
- Bot 本人权限、日历文件、Google / Outlook 链接、新加坡时区、UTC 等值时间、中文编码和过长内容回退。
- 密钥切换、所选群组与私聊范围、历史收集起点、数据清除。
- Windows 初始化、保留原密钥、文件缺失时停止、路径和 Docker 参数；权限与 Docker 调用使用模拟。

本轮日历网页测试 7 项通过；活动、Bot 和 API 三个后端包的单元与独立数据库测试通过。已完成网页构建、Linux x64 / ARM64 后端编译、Windows 脚本模拟测试和 Compose 配置检查。曾在 Apple Silicon Mac 上验证真实文字通知、模型处理、私人 Bot 推送和 `.ics` 附件发送。

仍需实测：Windows 和 Intel 安装、Docker 容器启动、真实编辑／删除及休眠后的完整回补、手机日历导入，以及登录 Google / Outlook 后的保存流程。图片识别尚未实现。模拟测试和接口回执不等同于这些实际操作已成功。
