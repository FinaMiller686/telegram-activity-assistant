# Windows 安装

[简体中文](WINDOWS.md) | [English](WINDOWS.en.md)

Windows 上用 Docker Desktop 启动，不用另外安装 Go、Node.js 或数据库。

## 先准备好

安装 [Docker Desktop](https://docs.docker.com/desktop/setup/install/windows-install/)，按提示启用 WSL 2，必要时重启电脑。打开 Docker Desktop，使用 WSL 2 和 Linux 容器。

到[项目首页](https://github.com/FinaMiller686/telegram-activity-assistant)点击 **Code → Download ZIP**，解压到电脑里的一个文件夹。请选择本机 NTFS 磁盘，不要放在 OneDrive 等自动同步的目录里。

## 第一次启动

在解压后的项目文件夹里打开 PowerShell，先运行：

```powershell
.\scripts\windows.ps1 setup
```

这一步会生成数据库密码和主密钥，保存在 `.env` 和 `.local/windows/master.key`。不需要自己编密码，也不要把这两个文件发给别人。

接着检查 Windows 的「设备加密」或 BitLocker。项目文件和 Docker 数据所在的盘都需要受到保护；Docker 的数据位置可以在 **Settings → Resources → Advanced** 查看。

确认后，打开项目里的 `.env`，把这一行改为：

```dotenv
TGTLDR_STORAGE_PROTECTED=true
```

这只是记录你已检查磁盘，不会替你开启加密。尚未确认时，先不要连接 Telegram。

然后运行：

```powershell
.\scripts\windows.ps1 start
```

第一次需要下载和编译，速度取决于网络和电脑。准备好后会自动打开 [http://localhost:3000](http://localhost:3000)。

在网页里设置访问密码、登录自己的 Telegram、绑定自己的 Bot，再选择要关注的群组。想接 AI，就到「系统配置 → 摘要引擎」填写自己的接口和密钥，测试保存后开启 AI。[详细设置](../使用说明.md)

## 以后怎么用

保持 Docker Desktop 运行，助手就可以在后台工作。可以在 Docker Desktop 中开启「登录电脑后启动」。手动停止过助手的话，需要再运行一次 `start`。

```powershell
.\scripts\windows.ps1 status
.\scripts\windows.ps1 stop
.\scripts\windows.ps1 start
```

睡眠、关机或断网时会暂停，恢复后尝试补回消息。手机仍然可以接收 Bot 推送，但电脑离线期间不会生成新结果。

Windows 还没有做实机安装测试，脚本模拟测试、配置检查和编译已通过。第一次建议先开一个群，确认能收到通知再增加其他群。[测试记录](activity/TESTS.md)

<details>
<summary>启动遇到问题</summary>

- **脚本无法运行**：确认文件来自本项目、看过内容后，可在当前 PowerShell 窗口运行 `Set-ExecutionPolicy -Scope Process RemoteSigned`。下载的脚本如仍被锁定，可在文件属性中解除锁定。学校或公司电脑受策略管理时，请联系管理员。
- **Docker Desktop is not ready**：先打开 Docker Desktop，等它启动完成。
- **Switch Docker Desktop to Linux containers**：切换到 Linux 容器，不能使用 Windows 容器模式。
- **Confirm encryption**：还没完成上面的磁盘检查或 `.env` 设置。
- **Master key is missing / invalid**：需要恢复原主密钥，不能新建一个代替。
- **网页打不开**：运行 `.\scripts\windows.ps1 logs` 看错误。端口被占用时，可在 `.env` 修改 `TGTLDR_HOST_WEB_PORT`，再启动。
- **本地模型连接失败**：容器访问 Windows 上的模型通常填 `http://host.docker.internal:11434/v1`，不要填容器自己的 `127.0.0.1`。

脚本需要 PowerShell 5.1 以上和 Compose 2.20 以上。更新 Docker Desktop 会一并更新 Compose。需要排查引擎模式时，可运行 `docker info --format '{{.OSType}}'`，应该得到 `linux`。

</details>

<details>
<summary>更新、备份和已有数据</summary>

更新前先运行 `stop`。用 Git 下载的项目可以运行 `git pull --ff-only`，然后 `start`；用 ZIP 下载的，替换程序文件时要保留 `.env` 和 `.local/windows/`。

数据库在 Docker 数据卷里，主密钥在项目文件夹里。备份需要同时考虑两者，分别保管；只有代码不能恢复活动数据，只有数据库也无法解开已加密的登录信息。丢失主密钥可能无法恢复账号配置。

`stop` 不会删除数据。不要运行 `docker compose down -v`，它会删除数据卷。也尽量保留项目目录名，改名可能让 Compose 连到一套新的空数据库。

`setup` 可以重复运行，会保留原设置和主密钥。如果以前已用通用 Compose 启动过，需要先备份并迁移原主密钥，不能直接换成新生成的密钥。

主密钥只挂给后端容器，不放进数据库卷或镜像。脚本限制本地文件权限，但 Compose 的文件挂载不会加密宿主机。有关磁盘位置和挂载方式，可查 [Docker WSL 2 说明](https://docs.docker.com/desktop/features/wsl/)和 [Compose secrets 说明](https://docs.docker.com/compose/how-tos/use-secrets/)。

</details>
