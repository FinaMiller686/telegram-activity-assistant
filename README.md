# Telegram 活动助手

[简体中文](README.md) | [English](README.en.md)

把 Telegram 群里的英文活动通知整理成中文，发到你自己的 Bot。收到后可以标记参加、稍后决定或已报名，也可以下载日历文件。

程序在自己的电脑上运行。Mac 使用本机安装脚本，Windows 使用 Docker Desktop。

## 能做什么

- 选择要关注的群组和频道，自动接收新消息，整理活动时间、地点、介绍和报名截止日期。
- 合并重复转发的活动，保留消息来源，提示时间冲突。
- 在网页或 Bot 里管理活动，查看未来 7 天的安排和想参加的活动。
- 每晚 21:00（新加坡时间）发送摘要，提醒快到报名截止的活动。
- 导出 Apple、Google Calendar 和 Outlook 可导入的 `.ics` 文件，也可打开 Google / Outlook 的预填页面。

可以连接本地模型或 OpenAI 兼容接口。不接模型时使用规则提取，能整理部分日期、地点和链接，但不会翻译完整通知。

## 在 Windows 上安装

先安装 [Docker Desktop](https://docs.docker.com/desktop/setup/install/windows-install/)，使用 WSL 2 和 Linux 容器。下载代码并解压，在项目目录打开 PowerShell：

```powershell
.\scripts\windows.ps1 setup
```

接着按 [Windows 安装说明](docs/WINDOWS.md)检查数据所在的磁盘，修改 `.env` 后启动：

```powershell
.\scripts\windows.ps1 start
```

脚本会构建并启动数据库、后端和网页，准备好后打开浏览器。不需要另外安装 Go、Node.js 或 PostgreSQL。

Windows 脚本已做模拟测试，Windows 权限、Docker 构建和真实 Telegram 登录还需在朋友的电脑上验证。

## 在 Mac 上安装

先安装 [Homebrew](https://brew.sh/)，然后打开终端，依次运行：

```bash
git clone https://github.com/FinaMiller686/telegram-activity-assistant.git
cd telegram-activity-assistant
brew install go node postgresql@17
./scripts/macos-native.sh build
./scripts/initialize-encrypted-storage.sh
./scripts/macos-native.sh start
```

需要 Go 1.25+、Node.js 24+ 和 PostgreSQL 17。Apple Silicon 已测试，Intel Mac 尚未实测。

打开 [http://localhost:3000](http://localhost:3000)，完成以下设置：

1. 设置网页访问密码。
2. 在 [my.telegram.org/apps](https://my.telegram.org/apps) 申请 API ID 和 API Hash，在本机网页登录自己的 Telegram 账号。
3. 用 [BotFather](https://t.me/BotFather) 创建 Bot，填写 Token，给 Bot 发 `/start`，绑定私人聊天。
4. 同步群组列表，为想关注的群组开启「消息保存」。
5. 如需 AI，在「系统配置 → 摘要引擎」填写接口、API Key 和模型，测试并保存，再到「活动助手」开启 AI。

密码、验证码和密钥都在本机填写。仓库不包含任何人的账号配置和群消息。

以后想在登录 Mac 后自动启动，运行：

```bash
./scripts/install-launch-agent.sh
```

安装目录请保留在原处。Mac 睡眠、关机或断网时会暂停监听，恢复后会尝试补回消息。停止服务、添加新群和常见问题见[使用说明](使用说明.md)。

## Docker

也提供 [Docker Compose 配置](docker-compose.yml)，但这条安装方式还没有实机测试。使用 Docker 前请阅读[使用说明中的 Docker 部分](使用说明.md#docker)，确认数据卷和密钥的存放方式。

## 目前的限制

图片只处理文字说明，还不能读海报或二维码。活动信息可能漏提或判断错误，参加前请点开原始消息核对。添加到日历后，群消息的后续修改不会自动更新到日历。

开启外部 AI 后，所选群组的文字会发送给你配置的模型服务。程序不收集私聊，不向群组发言，也不代填报名表。更多数据存放和删除说明见[使用说明](使用说明.md#数据存放和删除)。

## 项目来源

基于 [fr0der1c/tgtldr](https://github.com/fr0der1c/tgtldr) 的 `88e98fc` 版本修改，沿用 Telegram 登录、数据库和网页，增加了活动提取、去重、Bot 按钮和日历导出。

保留原项目的 [PolyForm Noncommercial 1.0.0 许可证](LICENSE)和版权声明。可在许可范围内用于非商业用途，商业使用需要另行授权；该许可证不是 OSI 认定的开源许可证。

[测试记录](docs/activity/TESTS.md)
