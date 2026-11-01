"use client";

import {
  createContext,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
} from "react";

export type Language = "zh-CN" | "en";

const languageCookieName = "tgtldr_language";
const languageCookieMaxAge = 60 * 60 * 24 * 365;
const useDOMEffect =
  typeof window === "undefined" ? useEffect : useLayoutEffect;

type DictShape<T> = {
  [K in keyof T]: T[K] extends string ? string : DictShape<T[K]>;
};

export const zh = {
  language: {
    label: "界面语言",
    hint: "设置网页语言，活动摘要仍使用中文。",
    zhCN: "中文",
    en: "English",
  },
} as const;

export const en = {
  language: {
    label: "Language",
    hint: "Sets the web interface language. Activity summaries remain in Chinese.",
    zhCN: "中文",
    en: "English",
  },
} satisfies DictShape<typeof zh>;

type I18nContextValue = {
  language: Language;
  setLanguage: (language: Language) => void;
  dict: DictShape<typeof zh>;
};

const I18nContext = createContext<I18nContextValue | null>(null);

const textTranslations = {
  "已过期或已取消推送": "Expired or no longer eligible for delivery",
  "摘要片段": "Digest part",
  "截止提醒": "Deadline reminder",
  "推送记录": "Delivery history",
  "Telegram 已接收": "Accepted by Telegram",
  "内容未变，未重复发送": "Unchanged; not sent again",
  "已被新版替代": "Replaced by a newer version",
  "发送失败，等待重试": "Send failed; waiting to retry",
  "发送结果未确认，将重试": "Send unconfirmed; will retry",
  "活动卡片": "Activity card",
  "部分推送等待重试": "Some deliveries are waiting to retry",
  "私人 Bot 已配置": "Private Bot configured",
  "私人 Bot 尚未就绪，请检查配置": "Private Bot not ready; check settings",
  "等待提取：": "Awaiting extraction:",
  "提取失败：": "Extraction failures:",
  "需手动重试：": "Manual retry needed:",
  "待推送：": "Pending deliveries:",
  "发送重试：": "Retrying sends:",
  "结果未确认：": "Unconfirmed sends:",
  "监听群组：": "Monitored groups:",
  "补拉超过 45 分钟未完成：": "History checks overdue by 45 minutes:",
  "各群最近补拉的最早完成时间：": "Oldest completed history check across groups:",
  "统计只覆盖已保存的文字；纯图片通知暂不识别，AI 也可能漏判。已接收表示 Telegram 确认收到了推送，不代表手机已读。": "Counts cover saved text only. Image-only notices are not recognized, and AI can miss activities. Accepted means Telegram acknowledged the send, not that it was read on your phone.",
  "下次重试：": "Next retry:",
  "已导出日历文件，时间按新加坡（UTC+8）。": "Calendar exported using Singapore time (UTC+8).",
  "日历文件使用新加坡时间（UTC+8），可导入 Apple 日历、Google Calendar 或 Outlook。": "This file uses Singapore time (UTC+8) and can be imported into Apple Calendar, Google Calendar, or Outlook.",
  "登录后，TGTLDR 会显示为一台已登录设备。": "After sign-in, TGTLDR appears as a signed-in device in Telegram.",
  "群组列表为空，可以重试同步。": "The group list is empty. Try syncing again.",
  "缺少开始或结束时间，请在网页活动卡补齐后导出。": "Enter the missing start or end time in the web activity card before exporting.",
  "暂无可导出的活动，需要完整的开始和结束时间。": "No events are ready to export. Both start and end times are required.",
  "日历文件可导入 Apple 日历、Google Calendar 或 Outlook。": "Import this file into Apple Calendar, Google Calendar, or Outlook.",
  "暂无时间完整的可导出活动。补齐开始和结束时间后再导出日历。": "No activities have complete times for export. Enter start and end times before exporting.",
  "这条活动缺少准确的开始或结束时间，暂时无法生成日历。请先核对原始消息，或在网页活动卡补齐时间后导出。": "This activity needs an exact start and end time before calendar export. Check the original message or enter the times in its web activity card.",
  "这条活动已不可用，请使用 /calendar 查看最新活动。": "This activity is no longer available. Use /calendar to view current activities.",
  "日历文件发送失败，请稍后重试。": "Could not send the calendar file. Please try again.",
  "正在生成日历文件…": "Generating calendar file\u2026",
  "原文未注明": "Not specified in the source",
  "原文未写年份，按消息发布时间推定，请核对": "The source omits the year; inferred from the message publication date. Please verify.",
  "消息收集起点：": "Message collection starts at: ",
  "选择需要监控的群组与频道；开启 AI 后，已监听群组会自动整理新文字。": "Choose groups and channels to monitor. With AI enabled, new text in monitored chats is organized automatically.",
  "AI 总结跟随活动助手的总开关，仅处理已启用消息保存的群组。": "AI summaries follow the Activities master switch and process only chats with message saving enabled.",
  "选择需要保存消息的群组，查看 AI 状态，或回补历史消息。": "Choose chats to save, check their AI status, or backfill message history.",
  "在活动助手中统一开关，开启后自动覆盖已监听群组。": "Manage the master switch in Activities. Enabling it covers monitored chats automatically.",
  "前往活动助手设置 AI": "Manage AI in Activities",
  "读取中": "Loading",

  "开启 AI 后，已监听群组的新文字和图片说明会发送至配置的模型；图片文件不会上传。关闭后使用本机规则。": "When AI is enabled, new text and captions from monitored chats are sent to the configured model. Image files are not uploaded. Disable AI to use local rules.",
 "模型平台拒绝当前网络环境。请按平台要求连接获准的校园网络或学校官方 VPN 后重试。": "The model provider does not support this network. Use a permitted campus network or official university VPN as required by the provider, then retry.",
 "模型平台未接受密钥，请检查密钥是否正确、有效。": "The provider did not accept the key. Check that it is correct and active.",
 "模型平台拒绝请求（403）。请检查账号、模型权限和平台允许的网络。": "The provider rejected the request (403). Check account permissions, model access, and permitted networks.",
 "模型或接口地址不存在（404），请核对平台提供的准确名称和地址。": "Model or endpoint not found (404). Check the exact model ID and endpoint.",
 "模型平台限流或额度不足（429），请检查平台用量后重试。": "Rate limit or quota exceeded (429). Check provider usage before retrying.",
 "模型返回空正文，请检查模型是否需要更高的输出限制或关闭推理模式。": "The model returned no final text. Check its output limit or reasoning mode.",
  "模型连接测试失败，请检查模型名称、密钥、额度和接口地址。": "Model connection failed. Check the model ID, key, quota, and endpoint.",
  "获取模型列表": "Fetch models",
  "正在获取模型...": "Fetching models...",
  "模型列表已获取，请选择模型后测试连接。": "Models fetched. Select a model, then test the connection.",
  "平台未返回模型，请手动填写模型名称。": "No models returned. Enter a model ID manually.",
  "查询模型和测试连接不会发送群消息；保存后到活动助手开启 AI。": "Model lookup and connection tests do not send group messages. Enable AI in Activities after saving.",
  "外部模型地址须使用 HTTPS，且不能包含用户名、密码或查询参数。": "External model endpoints require HTTPS without credentials or query parameters.",
  "更换模型服务商后，请重新输入该服务商的 API Key。": "After changing providers, enter that provider's API key again.",
  "请先在本机填写 API Key。": "Enter the API key locally first.",
  "无法获取模型列表，请检查地址和密钥，或手动填写平台提供的模型名称。": "Could not fetch models. Check the endpoint and key, or enter a model ID from your provider.",
  "助手加密存储不可用": "Assistant encrypted storage is unavailable",
  "尚未确认数据存储保护，账号连接、群组监听和 Bot 推送已暂停。请按对应系统的安装说明检查配置。": "Account connection, monitoring, and Bot delivery are paused until storage protection is confirmed. Follow the setup guide for your operating system.",
  "账号和消息保存在自己的电脑上。首次连接前，请按对应系统的安装说明设置数据保护。": "Accounts and messages are stored on your own computer. Before connecting, follow the setup guide for your operating system to protect the data.",
  "活动助手": "Activities",
  "按新加坡时间整理活动、报名与个人参加清单。": "Organize activities and registrations in Singapore time.",
  "活动列表": "Activities",
  "添加到 Apple 日历": "Add to Apple Calendar",
  "添加到日历": "Add to calendar",
  "导出 .ics 日历文件": "Export .ics calendar file",
  "Google Calendar": "Google Calendar",
  "Outlook（学校/工作）": "Outlook (work or school)",
  "Outlook.com（个人）": "Outlook.com (personal)",
  "已生成 .ics 文件，可导入 Apple 日历、Google Calendar 或 Outlook。": "The .ics file is ready to import into Apple Calendar, Google Calendar, or Outlook.",
  "请在打开的日历页面核对内容并保存。": "Review the event on the calendar page and save it.",
  "内容较长，请改用 .ics 文件导入。": "The event is too long for a link. Import the .ics file instead.",
  "请在日历中核对内容并确认保存。": "Review the event in your calendar and save it.",
  "选择活动，导出日历文件或打开 Google / Outlook 日历页面。": "Choose an event to export a calendar file or open a Google / Outlook draft.",
  "填写或修改时间": "Enter or change times",
  "日历开始时间（新加坡）": "Calendar start time (Singapore)",
  "日历结束时间（新加坡）": "Calendar end time (Singapore)",
  "请填写开始和结束时间，程序不会猜测时间。": "Enter start and end times; the assistant will not guess them.",
  "请检查日期和时间，结束时间须晚于开始时间。": "Check the dates and times; the end must be later than the start.",
  "将下载日历文件；请打开并确认添加。": "A calendar file will download; open it and confirm the import.",
  "关键词": "Keywords",
  "类别": "Category",
  "状态": "Status",
  "全部类别": "All categories",
  "待参加与已报名": "Going and registered",
  "新活动": "New",
  "我要参加": "I want to attend",
  "稍后决定": "Decide later",
  "不感兴趣": "Not interested",
  "已完成报名": "Registered",
  "已参加": "Attended",
  "未来 7 天": "Next 7 days",
  "暂无符合条件的活动。请先在群组页面开启监控。": "No matching activities. Enable monitoring on the Chats page.",
  "未命名活动": "Untitled event",
  "活动日期": "Event date",
  "报名截止": "Registration deadline",
  "主办方": "Organizer",
  "参与资格": "Eligibility",
  "着装要求": "Dress code",
  "需要报名": "Registration required",
  "是": "Yes",
  "否": "No",
  "报名链接": "Registration link",
  "未找到有效报名链接，请查看原始消息。": "No valid registration link found. Check the original message.",
  "这两场活动存在时间冲突。": "These events overlap.",
  "来源已删除或撤回，请核对。": "The source was deleted or withdrawn. Please verify.",
  "来源": "Sources",
  "查看原始消息": "View original message",
  "处理与提醒设置": "Processing and reminders",
  "启用 AI": "Enable AI",
  "本人 Telegram 用户 ID": "Your Telegram user ID",
  "每日摘要时间": "Daily digest time",
  "截止提醒提前小时": "Reminder lead time in hours",
  "原始消息保留天数": "Raw message retention in days",
  "模型和 Bot 在系统配置中设置。Bot 仅限本人操作。": "Configure the model and Bot in Settings. Only the owner can use the Bot.",
  "每日摘要历史": "Digest history",
  "每日摘要将在设定时间生成。": "The digest is generated at the scheduled time.",
  "系统状态": "System status",
  "重试失败提取": "Retry failed extraction",
  "删除历史数据": "Delete history",
  "将暂停所有群组监控，并删除消息、活动、状态、摘要和本地媒体。登录凭据保留。": "Pause monitoring and delete messages, activities, statuses, digests and local media. Login credentials are retained.",
  "输入 DELETE_HISTORY 确认": "Enter DELETE_HISTORY to confirm",
  "操作未完成，请检查配置与系统状态。": "Action failed. Check settings and system status.",
  "规则提取，请核对原始通知": "Extracted by rules; check the original message.",
  "已保存": "Saved",
  "在这里选择需要监控的群组与频道。原版群组 AI 摘要已停用，请使用活动助手。": "Choose the groups and channels to monitor. Legacy AI summaries are disabled; use Activities.",
  "AI 默认关闭；本地模型可不填 API Key。": "AI is off by default. A local model may not need an API key.",
  "一键重跑": "Retry all",
  "重跑中": "Retrying",
  "重跑详情": "Retry details",
  "Telegram 请求被拒绝，请检查目标聊天配置。": "Telegram rejected the request. Check the target chat settings.",
  "Telegram Bot 凭据无效。": "The Telegram Bot credentials are invalid.",
  "Telegram Bot 没有向目标聊天发送消息的权限。": "The Telegram Bot cannot send messages to the target chat.",
  "Telegram 通知发送受到限流。": "Telegram notification delivery is rate limited.",
  "Telegram 服务暂时不可用。": "Telegram is temporarily unavailable.",
  "Telegram 通知发送超时。": "Telegram notification delivery timed out.",
  "已完成": "Completed",
  "群组摘要": "Chat summary",
  "失败任务恢复": "Failed task recovery",
  "正在重跑失败任务": "Retrying failed tasks",
  "一键重跑失败任务": "Retry all failed tasks",
  "重跑全部失败摘要，保留成功结果；完成后重新生成并发送失败的每日总览。": "Retry all failed summaries, preserve successful results, then regenerate and send failed Daily Digests.",
  "已提交失败任务重跑，完成后会补发失败的每日总览。": "Failed tasks queued for retry. Failed Daily Digests will be regenerated and sent afterwards.",
  "重跑失败详情": "Retry failure details",
  "失败通知发送异常": "Failure notification delivery errors",
  "发送尝试次数": "Delivery attempts",
  "Telegram 通知发送失败": "Telegram notification delivery failed",
  "总览创建时该群摘要正文为空，请先重新生成单群摘要。": "This chat's summary was empty when the digest was created. Regenerate the chat summary first.",
  "重试发送": "Retry delivery",
  "未纳入当日总览": "Not included in this day's digest",
  "当日总览已创建，但未包含该群。可按最新群组配置重新生成并发送。": "This day's digest already exists without this chat. Regenerate and send it using the latest chat settings.",
  "重新生成并发送": "Regenerate and send",
  "已按最新群组配置提交重新生成，成功后将自动发送到 Telegram。": "Regeneration with the latest chat settings has started. The digest will be sent to Telegram on success.",
  "纳入每日总览": "Include in Daily Digest",
  "临时故障最多重试 4 次，依次等待 1、3、5、10 分钟；0 表示关闭自动重试。": "Retry temporary failures up to 4 times, waiting 1, 3, 5, and 10 minutes in order; 0 disables automatic retries.",
  "重试次数上限必须在 0 到 4 之间。": "Summary retry limit must be between 0 and 4.",
  "正在检查当前状态...": "Checking current status...",
  "正在进入首次配置...": "Opening first-time setup...",
  "正在进入登录...": "Opening login...",
  "正在进入后台...": "Opening dashboard...",
  "无法读取当前状态，正在进入首次配置...": "Could not read current status. Opening first-time setup...",
  "关闭": "Close",
  "关闭提示": "Dismiss notification",
  "必填": "Required",
  "没有匹配项": "No matches",
  "Telegram 群消息与每日摘要平台": "Telegram group messages and daily summary platform",
  "访问登录": "Access Login",
  "请输入你在首次设置中配置的访问密码。": "Enter the access password configured during first-time setup.",
  "访问密码": "Access password",
  "登录": "Log in",
  "登录失败。": "Login failed.",
  "无法读取当前状态。": "Could not read current status.",
  "Too long, don't read. 为你每天节省时间。": "Too long, don't read. Save time every day.",
  "群组": "Chats",
  "摘要": "Summaries",
  "系统配置": "Settings",
  "系统配置分类": "Settings categories",
  "安全": "Security",
  "保存当前设置": "Save current settings",
  "用于查找你与 Bot 的会话，不影响 Bot 实际发送到的目标。": "Used to find your conversations with the Bot. It does not affect the Bot's delivery target.",
  "用于查找与 Bot 的会话；不影响 Bot 实际发送到的目标。": "Used to find conversations with the Bot; it does not affect the Bot's delivery target.",
  "先在目标私聊中给 Bot 发一条消息，再点击“获取 Chat ID”自动绑定并保存。": "Send the Bot a message in the target private chat, then click \"Get Chat ID\" to bind and save it automatically.",
  "修改后请保存当前设置。": "Save the current settings after making changes.",
  "获取 Chat ID 会自动保存；其它 Bot 设置需要点击保存。": "Chat ID lookup saves automatically. Save other Bot settings manually.",
  "同步群组": "Sync chats",
  "删除账号": "Delete account",
  "添加 Telegram 账号": "Add Telegram account",
  "重新登录": "Sign in again",
  "确定删除这个 Telegram 账号吗？已有消息和摘要不会被删除。": "Delete this Telegram account? Existing messages and summaries will be kept.",
  "该账号的群组已同步。": "Chats for this account have been synced.",
  "Telegram 账号已删除。": "Telegram account deleted.",
  "已连接": "Connected",
  "未连接": "Not connected",
  "Bot 推送": "Bot Delivery",
  "启用中": "Enabled",
  "未启用": "Disabled",
  "在这里选择需要保存消息和生成摘要的群组，并调整每个群的摘要设置。": "Choose which chats should save messages and generate summaries, then tune each chat's summary settings.",
  "已同步群组": "Synced Chats",
  "最新": "Latest",
  "当前 Telegram 账号下可管理的群组与超级群组。": "Groups and supergroups available to the current Telegram account.",
  "所有已绑定 Telegram 账号下可管理的群组与超级群组。": "Groups and supergroups available to all connected Telegram accounts.",
  "已启用消息保存": "Message Saving Enabled",
  "运行中": "Running",
  "启用后，系统会实时保存该群的消息。": "When enabled, the system saves messages from this chat in real time.",
  "已启用 AI 总结": "AI Summary Enabled",
  "已配置": "Configured",
  "只有启用 AI 总结的群组才会参与每日摘要。": "Only chats with AI summaries enabled are included in daily summaries.",
  "群组列表": "Chat List",
  "使用账号": "Account used for this chat",
  "需要重新登录": "Sign-in required",
  "先查看每个群当前是否启用，再通过操作按钮调整摘要规则、补充群聊背景或回补历史消息。": "Review each chat's status, then use actions to adjust summary rules, add context, or backfill history.",
  "搜索群组": "Search chats",
  "按群名搜索": "Search by chat name",
  "群类型": "Chat type",
  "全部": "All",
  "超级群组": "Supergroup",
  "消息保存": "Message saving",
  "已启用": "Enabled",
  "AI 总结": "AI summary",
  "没有匹配的群组": "No matching chats",
  "调整筛选条件后再试一次。": "Adjust the filters and try again.",
  "群组名称": "Chat name",
  "操作": "Actions",
  "无公开用户名": "No public username",
  "最近 30 天消息": "Last 30 days of messages",
  "聊天记录": "Chat History",
  "返回群组列表": "Back to chats",
  "查看该群摘要": "View chat summaries",
  "查看摘要": "View summaries",
  "按日期查看群聊记录，向上滚动可加载当天更早的消息。": "Browse chat history by date. Scroll up to load earlier messages from the same day.",
  "正在读取聊天记录…": "Loading chat history...",
  "上一有消息日": "Previous active day",
  "下一有消息日": "Next active day",
  "选择日期": "Choose date",
  "打开日期选择器": "Open date picker",
  "选择聊天记录日期": "Choose chat history date",
  "聊天记录加载失败": "Could not load chat history",
  "当天没有聊天记录": "No messages on this day",
  "可以选择其他日期，或先为该群启用消息保存。": "Choose another date, or enable message saving for this chat first.",
  "正在加载更早消息…": "Loading earlier messages...",
  "加载更早消息": "Load earlier messages",
  "正在加载聊天记录": "Loading chat history",
  "未知发言人": "Unknown sender",
  "原消息未找到": "Original message not found",
  "图片消息": "Photo",
  "文件消息": "File",
  "媒体消息": "Media",
  "非文本消息": "Non-text message",
	"正在下载媒体…": "Downloading media...",
	"文件超过 100 MB 自动下载上限": "File exceeds the 100 MB automatic download limit",
	"媒体下载失败": "Media download failed",
	"仍要下载": "Download anyway",
	"重试": "Retry",
	"正在提交…": "Queuing...",
	"未知大小": "Unknown size",
  "非文本消息，无文字说明": "Non-text message without a caption",
  "搜索此群聊天记录": "Search this chat history",
  "搜索所有群组聊天记录": "Search all chat history",
  "搜索消息、发言人或 @username": "Search messages, senders, or @username",
  "搜索所有群组内的消息、发言人或 @username": "Search messages, senders, or @username across all chats",
  "清空聊天记录搜索": "Clear chat history search",
  "搜索": "Search",
  "当天摘要": "Summary for this day",
  "正在读取当天摘要": "Loading summary for this day",
  "正在读取当天摘要…": "Loading summary for this day...",
  "当天摘要加载失败": "Could not load summary for this day",
  "当天还没有摘要": "No summary for this day",
  "这个日期还没有生成摘要，可以前往摘要页手动生成。": "No summary has been generated for this date. You can generate one from the summaries page.",
  "摘要仍在处理中，请稍后再查看。": "The summary is still being processed. Check again later.",
  "在摘要页中查看": "View on summaries page",
  "展开聊天记录搜索": "Expand chat history search",
  "收起聊天记录搜索": "Collapse chat history search",
  "搜索聊天记录": "Search chat history",
  "收起搜索": "Collapse search",
  "应用群组过滤规则": "Apply chat filtering rules",
  "该群尚未配置消息过滤规则": "No message filtering rules are configured for this chat",
  "正在查看搜索结果上下文": "Viewing search result context",
  "返回搜索结果": "Back to search results",
  "查看当天最新消息": "View latest messages for this day",
  "搜索失败": "Search failed",
  "没有找到相关聊天记录": "No matching chat messages",
  "尝试减少关键词，或检查发言人名称和拼写。": "Try fewer keywords, or check the sender name and spelling.",
  "搜索结果": "Search Results",
  "查看上下文": "View context",
  "正在搜索聊天记录": "Searching chat history",
  "收起编辑": "Collapse editor",
  "编辑群组配置": "Edit chat settings",
  "收起历史消息回补": "Collapse history backfill",
  "加载历史消息": "Load history",
  "启用": "Enable",
  "停用": "Disable",
  "AI 总结交付方式": "AI summary delivery",
  "仅在网页端查看": "View in web app only",
  "通过 Bot 推送": "Deliver through Bot",
  "摘要时间": "Summary time",
  "模型 override": "Model override",
  "例如 gpt-4.1-mini": "e.g. gpt-4.1-mini",
  "验证机器人\n@verify_bot": "verification-bot\n@verify_bot",
  "验证机器人 @verify_bot": "verification-bot @verify_bot",
  "请完成入群验证\n验证已过期": "Please complete group verification\nVerification expired",
  "请完成入群验证 验证已过期": "Please complete group verification Verification expired",
  "例如：这个群主要讨论二级市场和链上项目；ATL 指 All Time Low；喊单通常是半开玩笑表达。": "Example: This group mainly discusses secondary markets and on-chain projects; ATL means All Time Low; \"calls\" are often half-joking trading ideas.",
  "告诉模型你希望如何总结这个群的消息，例如保留决策、行动项和重要链接。": "Tell the model how you want this chat summarized, such as preserving decisions, action items, and important links.",
  "保留机器人消息": "Keep bot messages",
  "保留": "Keep",
  "过滤": "Filter",
  "模型 override 留空时会跟随系统默认模型。": "When model override is empty, the system default model is used.",
  "过滤发言人": "Filtered senders",
  "每行一个，支持昵称或 @username，精确匹配。": "One per line. Supports display names or @username, matched exactly.",
  "过滤关键词": "Filtered keywords",
  "每行一个，按包含关系过滤消息内容。": "One per line. Messages are filtered by substring match.",
  "群聊背景": "Chat context",
  "补充群里常见黑话、术语、长期背景或默认语境，帮助模型正确理解讨论内容。": "Add jargon, terms, long-running context, or default assumptions so the model can understand the discussion.",
  "摘要额外要求": "Extra summary requirements",
  "保存该群配置": "Save chat settings",
  "回补后，这个群较早时间段的消息也可以用于生成摘要。": "After backfill, older messages from this chat can also be used for summaries.",
  "时间范围": "Time range",
  "最近 1 天": "Last 1 day",
  "最近 3 天": "Last 3 days",
  "最近 7 天": "Last 7 days",
  "最近 30 天": "Last 30 days",
  "自定义日期范围": "Custom date range",
  "开始日期": "Start date",
  "结束日期": "End date",
  "摘要记录": "Summary Records",
  "在这里搜索历史摘要、筛选状态，并在需要时手动补跑。": "Search historical summaries, filter by status, and run manual backfills when needed.",
  "累计": "Total",
  "已经写入数据库的摘要任务与结果。": "Summary tasks and results written to the database.",
  "生成成功": "Succeeded",
  "状态为 succeeded 的摘要数量。": "Number of summaries with succeeded status.",
  "正常": "Normal",
  "暂无": "None",
  "处理中": "Processing",
  "当前正在运行或等待完成的摘要。": "Summaries currently running or waiting to complete.",
  "空闲": "Idle",
  "生成失败": "Failed",
  "失败任务建议重新执行，并检查模型配置或上下文限制。": "Retry failed tasks and check model configuration or context limits.",
  "需排查": "Needs attention",
  "稳定": "Stable",
  "搜索摘要关键词": "Search summary keywords",
  "全部群组": "All chats",
  "生成状态": "Generation status",
  "全部状态": "All statuses",
  "成功": "Succeeded",
  "失败": "Failed",
  "等待中": "Pending",
  "发送状态": "Delivery status",
  "已发送": "Sent",
  "待发送": "Pending",
  "发送失败": "Delivery failed",
  "不发送": "Do not send",
  "手动补跑": "Manual run",
  "收起补跑": "Collapse manual run",
  "只会显示已启用 AI 总结的群组。": "Only chats with AI summaries enabled are shown.",
  "还没有可补跑的群组": "No chats available for manual run",
  "只有已启用 AI 总结的群组才会出现在这里。": "Only chats with AI summaries enabled appear here.",
  "选择群组": "Select chat",
  "日期": "Date",
  "立即生成": "Generate now",
  "没有匹配的摘要": "No matching summaries",
  "还没有摘要记录": "No summaries yet",
  "换个关键词或调整筛选条件后再试一次。": "Try another keyword or adjust filters.",
  "先展开手动补跑触发一次摘要，或者等待定时任务执行。": "Use manual run to generate a summary, or wait for the scheduled task.",
  "未知群组": "Unknown chat",
  "未记录模型": "Model not recorded",
  "上一页": "Previous",
  "下一页": "Next",
  "未发送": "Not sent",
  "当前群组设置为不通过 Bot 推送。": "This chat is configured not to deliver through Bot.",
  "Bot 配置尚未完成，当前无法发送。": "Bot configuration is incomplete, so this cannot be sent now.",
  "摘要尚未生成成功，当前不会执行发送。": "The summary has not succeeded yet, so delivery will not run.",
  "摘要已生成，等待自动发送或手动重试。": "The summary is ready and waiting for automatic delivery or manual retry.",
  "没有可查看的摘要": "No summary selected",
  "从列表中选择一条摘要后，这里会展示完整正文。": "Select a summary from the list to view the full content here.",
  "通过 Bot 发送": "Send through Bot",
  "查看原始 prompt": "View raw prompt",
  "重新生成": "Regenerate",
  "还没有摘要内容": "No summary content yet",
  "这条摘要还没有正文，请稍后重试或重新生成。": "This summary has no content yet. Try again later or regenerate it.",
  "服务器返回错误": "Server error",
  "OpenAI 请求参数": "OpenAI request parameters",
  "User prompt": "User prompt",
  "未安排自动重试。": "No automatic retry is scheduled.",
  "已达到自动重试上限。": "Automatic retry limit reached.",
  "自动重试已关闭。": "Automatic retry is disabled.",
  "复制": "Copy",
  "已复制": "Copied",
  "复制失败": "Copy failed",
  "原始 prompt": "Raw prompt",
  "查看这条摘要按当前规则重建出的完整 AI 输入上下文。": "View the full AI input context rebuilt for this summary using current rules.",
  "正在加载上下文预览…": "Loading context preview...",
  "暂时无法生成上下文预览": "Could not generate context preview",
  "稍后重试，或先确认这条摘要对应的消息仍然存在。": "Try again later, or confirm that the messages for this summary still exist.",
  "系统提示词": "System prompt",
  "合并提示词": "Merge prompt",
  "最终汇总阶段": "Final merge stage",
  "欢迎使用，请先完成设置向导": "Welcome. Complete the setup wizard first",
  "第 1 步": "Step 1",
  "第 2 步": "Step 2",
  "第 3 步": "Step 3",
  "第 4 步": "Step 4",
  "基础配置": "Basic Configuration",
  "Telegram 接入": "Telegram App",
  "Telegram App": "Telegram App",
  "用于登录你的 Telegram 账号并读取已加入群组的消息。": "TGTLDR signs in as a third-party Telegram client. Create a Telegram App first, then enter its API credentials here.",
  "请在 my.telegram.org 创建 Telegram App，然后填写 API ID 和 API Hash。": "Create a Telegram app at my.telegram.org, then enter its API ID and API Hash.",
  "申请 API ID / Hash": "Apply for API ID / Hash",
  "OpenAI 摘要": "OpenAI Summary",
  "用于生成每日摘要内容。": "Used to generate daily summary content.",
  "创建 API Key": "Create API Key",
  "测试连接": "Test connection",
  "测试中...": "Testing...",
  "OpenAI 连接测试成功。": "OpenAI connection test succeeded.",
  "自定义模型名": "Custom model name",
  "如果你使用兼容 OpenAI 的其他服务，可以手动填写模型名。": "If you use another OpenAI-compatible service, enter the model name manually.",
  "高级参数": "Advanced parameters",
  "输出长度": "Output length",
  "自动模式会让系统不显式限制输出长度，自定义模式才会传 Max Output Tokens。": "Auto mode does not set an explicit output limit. Custom mode applies the Max Output Tokens limit.",
  "自动模式不设置显式输出上限；自定义模式会应用 Max Output Tokens 限制。": "Auto mode does not set an explicit output limit. Custom mode applies the Max Output Tokens limit.",
  "自动": "Auto",
  "自定义": "Custom",
  "调用方式": "Request mode",
  "流式": "Streaming",
  "非流式": "Non-streaming",
  "流式更适合转发站，可降低网关等待完整响应导致的超时风险。": "Streaming works better with relay gateways and lowers the risk of gateway timeouts while waiting for the full response.",
  "摘要并行度": "Concurrent summaries",
  "并发摘要数": "Concurrent summaries",
  "当一天消息被拆成多个分块时，最多同时生成多少个阶段摘要。": "Maximum number of chunks to summarize at the same time.",
  "最多同时总结多少个消息分块。": "Maximum number of chunks to summarize at the same time.",
  "重试次数上限": "Retry limit",
  "保存并继续": "Save and continue",
  "登录 Telegram": "Log in to Telegram",
  "为了登录并获取消息，我们需要登录你的 Telegram 账号。登录后，": "To read messages, TGTLDR needs to log in to your Telegram account. After login,",
  "TGTLDR 会体现为你的一台已登录设备。": "TGTLDR appears as one of your logged-in devices.",
  "国家码": "Country code",
  "手机号": "Phone number",
  "返回上一步": "Back",
  "继续": "Continue",
  "输入验证码": "Enter verification code",
  "验证码": "Verification code",
  "输入 Telegram 发来的验证码": "Enter the code from Telegram",
  "重新发送验证码": "Resend code",
  "返回修改手机号": "Change phone number",
  "输入两步验证密码": "Enter two-step verification password",
  "两步验证密码": "Two-step verification password",
  "输入你的两步验证密码": "Enter your two-step verification password",
  "已登录并同步群组": "Logged in and chats synced",
  "已登录，尚未同步群组": "Logged in, chats not synced yet",
  "当前还没有拿到群组列表。若你已经加入了 Telegram": "No chat list is available yet. If you have joined Telegram",
  "群组，可以重试一次同步。": "groups, you can try syncing again.",
  "登录账号": "Account",
  "已授权账号": "Authorized account",
  "发现群组": "Discovered chats",
  "重试同步群组": "Retry chat sync",
  "同时通过 Telegram Bot 推送": "Also deliver through Telegram Bot",
  "启用 Bot 推送后必须填写。": "Required when Bot delivery is enabled.",
  "目标 Chat ID": "Target Chat ID",
  "先给 Bot 发消息，再点击“获取 Chat ID”自动绑定。": "Send the Bot a message first, then click \"Get Chat ID\" to bind automatically.",
  "获取 Chat ID": "Get Chat ID",
  "正在获取...": "Fetching...",
  "尚未绑定 Chat ID": "Chat ID not bound yet",
  "获取成功后会显示在这里，并在“保存并完成”时一起保存。": "After successful lookup, it appears here and is saved when setup is completed.",
  "保存并完成": "Save and finish",
  "设置访问密码": "Set access password",
  "访问保护": "Access protection",
  "确认访问密码": "Confirm access password",
  "两次输入的密码不一致。": "The two passwords do not match.",
  "设置密码并继续": "Set password and continue",
  "默认时区": "Default timezone",
  "选择默认时区": "Select default timezone",
  "搜索时区，例如 Asia/Shanghai": "Search timezone, e.g. Asia/Shanghai",
  "没有匹配的时区": "No matching timezones",
  "管理 Telegram 接入、摘要引擎和 Bot 推送。": "Manage your Telegram app credentials, summary engine, preferences, and Bot delivery.",
  "管理 Telegram App、摘要引擎、偏好设置和 Bot 推送。": "Manage your Telegram app credentials, summary engine, preferences, and Bot delivery.",
  "在这里管理 Telegram 接入、摘要模型、默认时区和 Bot 推送。": "Manage your Telegram app credentials, summary engine, preferences, and Bot delivery.",
  "在这里管理 Telegram App、摘要引擎、偏好设置和 Bot 推送。": "Manage your Telegram app credentials, summary engine, preferences, and Bot delivery.",
  "摘要引擎": "Summary Engine",
  "这些参数决定摘要模型、接口地址、输出策略和并行处理方式。": "Configure the model, API endpoint, output length, and parallel processing.",
  "配置模型、API 地址、输出长度和并行处理方式。": "Configure the model, API endpoint, output length, and parallel processing.",
  "系统行为": "Preferences",
  "偏好设置": "Preferences",
  "这些设置会影响摘要日期计算和系统调度行为。": "These settings control the timezone used for summary dates and schedules.",
  "这些设置会影响摘要日期计算、网页文案和默认输出语言。": "These settings control the timezone used for summary dates, schedules, and the default output language.",
  "这些设置会控制摘要日期和定时任务使用的时区。": "These settings control the timezone used for summary dates and schedules.",
  "这些设置会控制时区、语言和消息附件的下载行为。": "These settings control the timezone, language, and message attachment download behavior.",
  "自动下载消息附件": "Automatically download message attachments",
  "关闭后，新附件和尚未开始的附件会等待手动下载；头像不受影响。": "When disabled, new and queued attachments wait for manual download. Avatars are unaffected.",
  "这些设置会控制摘要日期、定时任务和默认输出语言。": "These settings control the timezone used for summary dates, schedules, and the default output language.",
  "Telegram 账号": "Telegram Account",
  "请输入要登录的 Telegram 手机号。": "Enter the phone number for the Telegram account you want to sign in to.",
  "在这里完成登录、重新登录或重新同步群组。": "Log in, log in again, or resync chats here.",
  "初始化完成后，后台页面和 API 都需要使用这个密码登录。": "After initialization, dashboard pages and APIs require this password.",
  "用于登录 Telegram 账号并读取群组消息。": "TGTLDR signs in as a third-party Telegram client. Create a Telegram App first, then enter its API credentials here.",
  "在 my.telegram.org/apps 申请后获得。": "Created at my.telegram.org/apps.",
  "在 my.telegram.org/apps 创建后获得。": "Created at my.telegram.org/apps.",
  "已保存时会显示掩码。留空表示保持现有值。": "Saved secrets are masked. Leave blank to keep the current value.",
  "建议范围 0.0 - 2.0；摘要场景通常建议 0.1 - 0.7。": "Suggested range: 0.0-2.0. Summaries usually work best around 0.1-0.7.",
  "建议范围：0.0-2.0。摘要场景通常建议 0.1-0.7。": "Suggested range: 0.0-2.0. Summaries usually work best around 0.1-0.7.",
  "当前密码": "Current password",
  "新密码": "New password",
  "至少 8 位。": "At least 8 characters.",
  "至少 8 位。后续可以在系统配置中修改。": "At least 8 characters. You can change it later in Settings.",
  "确认新密码": "Confirm new password",
  "更新访问密码": "Update access password",
  "退出登录": "Log out",
  "Telegram Bot 推送": "Telegram Bot Delivery",
  "是否启用": "Delivery mode",
  "投递方式": "Delivery mode",
  "关闭 Bot 推送": "Web app only",
  "开启 Bot 推送": "Send via Telegram Bot",
  "仅网页端查看": "Web app only",
  "通过 Telegram Bot 推送": "Send via Telegram Bot",
  "如果你只在网页端看摘要，这一块可以保持关闭。": "Keep this off if you only read summaries in the web app.",
  "先给 Bot 发消息，再点击“获取 Chat ID”自动绑定并保存。": "Send a message to the Bot first, then click \"Get Chat ID\" to bind the target chat automatically.",
  "1. 先在目标私聊或群聊里给 Bot 发一条消息。": "1. Send a message to the Bot in the target private chat or group.",
  "2. 回到这里点击“获取 Chat ID”。": "2. Return here and click \"Get Chat ID\".",
  "自动获取前需要先完成上一步的 Telegram 登录。": "Complete the previous Telegram login step before automatic lookup.",
  "自动获取前需要先完成上面的 Telegram 登录。": "Complete the Telegram login above before automatic lookup.",
  "正在保存...": "Saving...",
  "获取成功后会自动保存并立即显示在这里。": "After successful lookup, it is saved automatically and shown here.",
  "如果你只想在网页端查看摘要，可以把 Bot 推送保持关闭。": "Keep Bot delivery off if you only read summaries in the web app.",
  "获取 Chat ID 会自动保存；其它系统配置修改仍需在这里统一保存。": "Chat ID binding is saved automatically. Save other settings with the button below.",
  "保存系统配置": "Save settings",
  "当前账号": "Current account",
  "已自动绑定并保存 Chat ID。": "Chat ID bound and saved automatically.",
  "连接状态": "Connection status",
  "重新同步群组": "Resync chats",
  "重新登录 Telegram": "Reconnect Telegram",
  "发送验证码": "Send code",
  "取消": "Cancel",
  "完成登录": "Complete login",
  "留空表示保持现有值": "Leave empty to keep the current value",
  "私聊": "Private chat",
  "群聊": "Group",
  "频道": "Channel",
  "会话": "Chat",
  "超级群": "Supergroup",
  "协调世界时": "Coordinated Universal Time",
  "界面语言": "Language",
  "设置网页语言，活动摘要仍使用中文。": "Sets the web interface language. Activity summaries remain in Chinese.",
  "仅在网页端查看摘要": "View summaries in the web app only",
  "在这里搜索和筛选摘要记录；点开某条摘要后，会从右侧展开完整正文。": "Search and filter summary records here. Select a summary to open the full content from the right.",
  "进行中": "In progress",
  "筛选": "Filters",
  "收起筛选": "Hide filters",
  "上下文长度": "Context window",
  "自动模式会识别已知模型；自定义模型或转发服务可手动填写实际上下文窗口。": "Auto detects known models. Enter the actual context window manually for custom models or compatible providers.",
  "自动识别": "Auto detect",
  "请选择有效的上下文长度模式。": "Select a valid context window mode.",
  "自定义上下文长度不能小于 4096 tokens。": "Custom context window must be at least 4096 tokens.",
  "很久没看摘要了吗？开始快速回顾一下吧": "Haven’t checked your summaries in a while? Catch up quickly.",
  "选择一段时间和群组，把散落的每日摘要整理成一份阶段回顾。": "Choose a period and chats, then turn daily summaries into one focused review.",
  "开始快速回顾": "Start Catch Up",
  "快速回顾": "Catch Up",
  "从已有的每日摘要中，快速整理出一份阶段回顾。": "Turn existing daily summaries into a quick period review.",
  "查看之前的快速回顾": "View previous Catch Ups",
  "下一步": "Next",
  "上一步": "Back",
  "开始生成": "Start generating",
  "选择时间周期": "Choose a time period",
  "固定周期均截止到昨天，不会混入尚未结束的今天。": "Preset periods end yesterday and never include the current incomplete day.",
  "最多 90 天": "Up to 90 days",
  "全选": "Select all",
  "清空": "Clear",
  "将结果发送到我的 Telegram": "Send the result to my Telegram",
  "此功能要求先配置 Telegram Bot": "Configure Telegram Bot before using this option",
  "正在生成快速回顾…": "Generating Catch Up…",
  "还没有生成过快速回顾。": "No Catch Up has been generated yet.",
  "正在读取…": "Loading…",
  "生成完成后，可以在这里查看完整阶段回顾。": "The complete period review will appear here after generation.",
  "没有可查看的快速回顾": "No Catch Up to view",
  "已重新提交 Telegram 发送。": "Telegram delivery resubmitted.",
  "重新发送": "Send again",
  "任务会在后台继续，完成后这里会自动刷新。": "The task continues in the background and this view refreshes automatically.",
  "来源摘要": "Source summaries",
  "点击日期可打开对应的每日摘要。": "Select a date to open its daily summary.",
  "快速回顾已生成完成。": "Catch Up is ready.",
  "快速回顾生成失败。": "Catch Up generation failed.",
  "快速回顾历史记录": "Catch Up history",
  "加载更多": "Load more",
  "每日总览": "Daily Digest",
  "一天一篇，看完所有群": "One digest a day for all your chats",
  "将所有参与推送的群组摘要合并成一篇，每天只接收一次 Telegram 推送。": "Combine all participating chat summaries into one message and receive just one Telegram delivery each day.",
  "正在读取每日总览状态": "Loading Daily Digest status",
  "等待 Bot 配置": "Waiting for Bot setup",
  "查看每日总览": "View Daily Digests",
  "启用每日总览": "Enable Daily Digest",
  "查看历史总览": "View digest history",
  "启用每日总览？": "Enable Daily Digest?",
  "有可汇总内容时，每天只会收到一篇跨群总览。": "When there is content to summarize, you will receive one cross-chat digest for the day.",
  "开启后，默认纳入已启用摘要的群组，等待摘要完成后合并推送；可以逐群取消。": "Enabled chat summaries are included by default and combined when ready. You can exclude individual chats.",
  "同时启用 Telegram Bot": "Also enable Telegram Bot",
  "Telegram Bot 当前未启用，确认后会一并开启。": "Telegram Bot is currently disabled and will also be enabled after confirmation.",
  "设置从今天的消息开始生效，之前的摘要不会改变。": "This setting applies starting with today's messages. Earlier summaries will not change.",
  "暂不开启": "Not now",
  "正在启用…": "Enabling…",
  "确认启用": "Enable",
  "每日总览已启用，将从今天的消息开始生效。": "Daily Digest is enabled and will apply starting with today's messages.",
  "前往 Bot 设置": "Open Bot settings",
  "每日总览需要 Telegram Bot 才能发送。": "Daily Digest requires Telegram Bot delivery.",
  "先完成 Bot 配置": "Complete Bot setup first",
  "请先配置 Bot Token 和目标 Chat ID，完成后再回来启用每日总览。": "Configure a Bot Token and target Chat ID, then return to enable Daily Digest.",
  "每日总览历史记录": "Daily Digest history",
  "启用后，每天生成的总览会保存在这里。": "Daily Digests will be saved here after you enable the feature.",
  "没有可查看的每日总览": "No Daily Digest to view",
  "正在生成每日总览…": "Generating Daily Digest…",
  "点击群组可打开对应的单群每日摘要。": "Select a chat to open its daily summary.",
  "未纳入": "Omitted",
  "无需发送": "No delivery needed",
  "当天没有可推送的新内容。": "There was no new content to deliver that day.",
  "摘要推送形式": "Summary delivery format",
  "开启后默认纳入已启用摘要的群组，可逐群取消；设置从保存当天生效。": "Enabled chat summaries are included by default. You can exclude individual chats. Changes apply from the day you save.",
  "参与 Bot 推送的群组会在各自摘要完成后分别发送。修改从保存当天的消息开始生效。": "Each participating chat is sent separately after its summary finishes. Changes apply starting with messages from the day they are saved.",
  "按群分别发送": "Send each chat separately",
  "每日总览（合并发送）": "Daily Digest (combined)",
  "Telegram 推送": "Telegram delivery",
  "不参与，仅网页查看": "Do not include; web only",
  "参与推送": "Include in delivery",
  "未纳入总览": "Not included",
  "单群摘要生成失败，无法纳入每日总览。": "This chat summary failed and could not be included in the Daily Digest.",
  "当日无消息": "No messages that day",
  "该群当天没有新消息，已完成等待但未进入总览正文。": "This chat had no new messages that day, so it completed the wait without entering the digest body.",
  "已归入总览": "Included in digest",
  "该摘要已由每日总览统一处理。": "This summary has been handled by the Daily Digest.",
  "等待总览": "Waiting for digest",
  "等待其它参与群组完成后统一生成每日总览。": "Waiting for the other participating chats before generating the Daily Digest.",
  "Bot 配置尚未完成，暂时无法生成每日总览。": "Bot setup is incomplete, so the Daily Digest cannot be generated yet.",
  "总览生成失败": "Digest generation failed",
  "该摘要已纳入每日总览，但总览生成失败。": "This summary was included, but Daily Digest generation failed.",
  "每日总览正在处理该摘要。": "The Daily Digest is processing this summary.",
  "总览发送失败": "Digest delivery failed",
  "总览待发送": "Digest pending delivery",
  "每日总览已经生成，等待统一发送。": "The Daily Digest is ready and waiting to be delivered.",
  "总览待手动发送": "Digest awaiting manual delivery",
  "每日总览已由用户重新生成，需要手动确认发送。": "The Daily Digest was regenerated and requires manual delivery confirmation.",
  "等待手动发送": "Waiting for manual delivery",
  "重新生成完成后不会自动重复发送。": "A regenerated Daily Digest is not sent again automatically.",
} as const;

const attributeNames = ["placeholder", "title", "aria-label"] as const;

const dynamicTranslations: Array<[RegExp, (...matches: string[]) => string]> = [
  [/^第 (\d+) 步，共 (\d+) 步$/, (current, total) => `Step ${current} of ${total}`],
  [/^有 (\d+) 个群组正在使用此账号，请先为这些群组选择其他账号。$/, (count) => `${count} chats are using this account. Choose another account for them before deleting it.`],
  [/^已保存「(.+)」的配置。$/, (chat) => `Saved settings for "${chat}".`],
  [/^正在为「(.+)」回补 (.+) 到 (.+) 的历史消息。$/, (chat, from, to) => `Backfilling history for "${chat}" from ${from} to ${to}.`],
  [/^「(.+)」历史消息回补完成，已处理 (\d+) 条消息。$/, (chat, count) => `History backfill for "${chat}" completed. Processed ${count} messages.`],
  [/^「(.+)」历史消息回补失败：(.+)$/, (chat, error) => `History backfill for "${chat}" failed: ${error}`],
  [/^「(.+)」历史消息回补失败。$/, (chat) => `History backfill for "${chat}" failed.`],
  [/^将回补 (.+) 到 (.+) 的消息。$/, (from, to) => `Will backfill messages from ${from} to ${to}.`],
  [/^当前群类型：(.+)$/, (type) => `Current chat type: ${translateExact(type)}`],
  [/^(.+)：(\d+) 条消息$/, (date, count) => `${date}: ${count} messages`],
  [/^(.+) · (\d+) 条消息 · (.+)$/, (date, count, timezone) => `${date} · ${count} messages · ${timezone}`],
  [/^已加载当天全部 (\d+) 条消息$/, (count) => `All ${count} messages for this day are loaded`],
  [/^已加载当天全部 (\d+) 条消息，$/, (count) => `All ${count} messages for this day are loaded, `],
  [/^(\d+) 条消息$/, (count) => `${count} messages`],
  [/^共 (\d+) 条$/, (count) => `${count} results`],
  [/^(.+) 聊天记录$/, (date) => `Chat history for ${date}`],
  [/^消息 (\d+) 条 · 分块\s*(\d+)$/, (messages, chunks) => `${messages} messages · ${chunks} chunks`],
  [/^(.+) · 消息 (\d+) 条 · 分块\s*(\d+)$/, (model, messages, chunks) => `${model} · ${messages} messages · ${chunks} chunks`],
  [/^阶段提示词 1 段 · 消息 (\d+) 条 · 分块 (\d+)$/, (messages, chunks) => `1 stage prompt · ${messages} messages · ${chunks} chunks`],
  [/^字符 (\d+)$/, (count) => `${count} characters`],
  [/^消息 (\d+) 条 · 字符 (\d+)$/, (messages, chars) => `${messages} messages · ${chars} characters`],
  [/^步骤 (\d+)\/(\d+)$/, (current, total) => `Step ${current}/${total}`],
  [/^第 (\d+) \/ (\d+) 页$/, (page, total) => `Page ${page} / ${total}`],
  [/^共 (\d+) 条摘要$/, (total) => `${total} summaries`],
  [/^第 (\d+) 页，共 (\d+) 页$/, (page, total) => `Page ${page} of ${total}`],
  [/^已发送于 (.+)$/, (time) => `Sent at ${time}`],
  [/^验证码已发送到 (.+)。$/, (phone) => `Verification code sent to ${phone}.`],
  [/^Telegram 暂时限制了请求，请在 (\d+) 秒后重试。$/, (seconds) => `Telegram temporarily limited requests. Try again in ${seconds} seconds.`],
  [/^配置已保存。$/, () => "Settings saved."],
  [/^已提交摘要生成任务。$/, () => "Summary generation task submitted."],
  [/^已提交通过 Bot 发送。$/, () => "Bot delivery submitted."],
  [/^已提交重新生成。$/, () => "Regeneration submitted."],
  [/^系统配置已保存。$/, () => "System settings saved."],
  [/^访问密码至少需要 8 位。$/, () => "Access password must be at least 8 characters."],
  [/^两次输入的访问密码不一致。$/, () => "The two access passwords do not match."],
  [/^访问密码已更新。$/, () => "Access password updated."],
  [/^该账号开启了两步验证，请继续输入密码。$/, () => "This account has two-step verification enabled. Continue by entering the password."],
  [/^Telegram 登录成功。$/, () => "Telegram login succeeded."],
  [/^两步验证通过，Telegram 登录成功。$/, () => "Two-step verification passed. Telegram login succeeded."],
  [/^Telegram 登录成功，群组已同步。$/, () => "Telegram login succeeded and chats have been synced."],
  [/^两步验证通过，Telegram 登录成功，群组已同步。$/, () => "Two-step verification passed. Telegram login succeeded and chats have been synced."],
  [/^(.+) 已同步 (\d+) 个群组。$/, (prefix, count) => `${translateText(prefix, "en")} Synced ${count} chats.`],
  [/^(.+) 当前还没有拿到群组列表，你可以稍后重试同步。$/, (prefix) => `${translateText(prefix, "en")} No chat list is available yet. You can retry syncing later.`],
  [/^已重新同步 (\d+) 个群组。$/, (count) => `Resynced ${count} chats.`],
  [/^已重新同步，但当前没有发现可管理的群组。$/, () => "Resynced, but no manageable chats were found."],
  [/^已同步 (\d+) 个群组。$/, (count) => `Synced ${count} chats.`],
  [/^(.+) 当前没有发现可管理的群组。$/, (prefix) => `${translateText(prefix, "en")} No manageable chats were found.`],
  [/^已同步，但当前没有发现可管理的群组。$/, () => "Synced, but no manageable chats were found."],
  [/^当前已绑定：(.+)$/, (chatId) => `Currently bound: ${chatId}`],
  [/^当前将绑定：(.+)$/, (chatId) => `Will bind: ${chatId}`],
  [/^自动获取前请先完成 Telegram 登录。$/, () => "Complete Telegram login before automatic lookup."],
  [/^请先填写 Bot Token。$/, () => "Enter the Bot Token first."],
  [/^未找到最近消息，请先给 Bot 发一条消息后再重试。$/, () => "No recent message was found. Send the Bot a message first, then retry."],
  [/^已获取目标 Chat ID，点击“保存并完成”后生效。$/, () => "Target Chat ID found. It takes effect after you click Save and finish."],
  [/^找到了多个可能的会话，请选择一个。$/, () => "Multiple possible chats were found. Select one."],
  [/^已选择目标 Chat ID，点击“保存并完成”后生效。$/, () => "Target Chat ID selected. It takes effect after you click Save and finish."],
  [/^启用 Bot 推送时必须填写 Bot Token。$/, () => "Bot Token is required when Bot delivery is enabled."],
  [/^基础配置已保存，进入登录步骤。$/, () => "Basic configuration saved. Continue to login."],
  [/^配置已保存，正在进入后台。$/, () => "Settings saved. Opening dashboard."],
  [/^访问密码已设置，继续完成基础配置。$/, () => "Access password set. Continue with basic configuration."],
  [/^请填写 Telegram API ID。$/, () => "Enter Telegram API ID."],
  [/^请填写 Telegram API Hash。$/, () => "Enter Telegram API Hash."],
  [/^请填写 OpenAI API Key。$/, () => "Enter OpenAI API Key."],
  [/^请填写 Model。$/, () => "Enter Model."],
  [/^请选择有效的输出长度模式。$/, () => "Select a valid output length mode."],
  [/^自定义输出长度时，请填写有效的 Max Output Tokens。$/, () => "Enter valid Max Output Tokens when output length is custom."],
  [/^请选择有效的调用方式。$/, () => "Select a valid request mode."],
  [/^调用方式必须是 stream 或 non_stream。$/, () => "Request mode must be stream or non_stream."],
  [/^摘要并行度必须在 1 到 6 之间。$/, () => "Summary parallelism must be between 1 and 6."],
  [/^重试次数上限不能小于 0。$/, () => "Retry limit cannot be less than 0."],
  [/^重试起始间隔必须至少为 1 分钟。$/, () => "Initial retry delay must be at least 1 minute."],
  [/^退避倍率必须大于或等于 1。$/, () => "Backoff multiplier must be at least 1."],
  [/^已自动重试 (.+)$/, (count) => `Automatic retries ${count}`],
  [/^下次自动重试：(.+)$/, (time) => `Next automatic retry: ${time}`],
  [/^最近一次：(.+)$/, (date) => `Latest: ${date}`],
  [/^(\d+) 个群组 · (\d+) 份摘要$/, (chats, summaries) => `${chats} chats · ${summaries} summaries`],
  [/^(\d+) 个群组当天没有新消息。$/, (chats) => `${chats} chats had no new messages that day.`],
];

export function normalizeLanguage(language: string | null | undefined): Language {
  return language === "en" ? "en" : "zh-CN";
}

export function detectBrowserLanguage(): Language {
  if (typeof navigator === "undefined") {
    return "zh-CN";
  }
  return navigator.language.toLowerCase().startsWith("en") ? "en" : "zh-CN";
}

export function translateText(text: string, language: Language) {
  if (language !== "en") {
    return text;
  }
  const exact = translateExact(text);
  if (exact !== text) {
    return exact;
  }
  for (const [pattern, render] of dynamicTranslations) {
    const match = text.match(pattern);
    if (!match) {
      continue;
    }
    return render(...match.slice(1));
  }
  return text;
}

function translateExact(text: string) {
  const trimmed = text.trim();
  const translated = textTranslations[trimmed as keyof typeof textTranslations];
  if (!translated) {
    return text;
  }
  return text.replace(trimmed, translated);
}

type I18nProviderProps = PropsWithChildren<{
  initialLanguage?: Language;
}>;

export function I18nProvider({
  children,
  initialLanguage,
}: I18nProviderProps) {
  const [language, setLanguageState] = useState<Language>(() =>
    normalizeLanguage(
      initialLanguage ?? readCookieLanguage() ?? detectBrowserLanguage(),
    ),
  );

  const setLanguage = useCallback((nextLanguage: Language) => {
    const normalized = normalizeLanguage(nextLanguage);
    persistCookieLanguage(normalized);
    setLanguageState(normalized);
  }, []);

  useEffect(() => {
    if (initialLanguage) {
      return;
    }
    setLanguage(readCookieLanguage() ?? detectBrowserLanguage());
  }, [initialLanguage, setLanguage]);

  const dict = useMemo(() => (language === "en" ? en : zh), [language]);
  const value = useMemo<I18nContextValue>(
    () => ({
      language,
      setLanguage,
      dict,
    }),
    [dict, language, setLanguage],
  );

  return (
    <I18nContext.Provider value={value}>
      <DOMTranslator language={language} />
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return value;
}

function DOMTranslator({ language }: { language: Language }) {
  useDOMEffect(() => {
    document.documentElement.lang = language;
    translateDocument(language);
    document.documentElement.classList.remove("i18n-pending");
    const observer = new MutationObserver(() => translateDocument(language));
    observer.observe(document.body, {
      childList: true,
      subtree: true,
      characterData: true,
      attributes: true,
      attributeFilter: [...attributeNames],
    });
    return () => observer.disconnect();
  }, [language]);

  return null;
}

function readCookieLanguage(): Language | null {
  if (typeof document === "undefined") {
    return null;
  }
  const cookie = document.cookie
    .split("; ")
    .find((part) => part.startsWith(`${languageCookieName}=`));
  if (!cookie) {
    return null;
  }
  return normalizeLanguage(decodeURIComponent(cookie.split("=")[1] ?? ""));
}

function persistCookieLanguage(language: Language) {
  if (typeof document === "undefined") {
    return;
  }
  document.cookie = [
    `${languageCookieName}=${encodeURIComponent(language)}`,
    "path=/",
    `max-age=${languageCookieMaxAge}`,
    "samesite=lax",
  ].join("; ");
}

const originalText = new WeakMap<Text, string>();
const translatedText = new WeakMap<Text, string>();

function translateDocument(language: Language) {
  translateTextNodes(document.body, language);
  translateAttributes(document.body, language);
}

function translateTextNodes(root: HTMLElement, language: Language) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let node = walker.nextNode() as Text | null;
  while (node) {
    if (shouldSkipNode(node)) {
      node = walker.nextNode() as Text | null;
      continue;
    }
    const current = node.nodeValue ?? "";
    const previousTranslated = translatedText.get(node);
    let original = originalText.get(node) ?? current;
    if (!originalText.has(node) || (previousTranslated !== undefined && current !== previousTranslated)) {
      original = current;
      originalText.set(node, original);
    }
    if (!originalText.has(node)) {
      originalText.set(node, original);
    }
    const translated = translateText(original, language);
    if (node.nodeValue !== translated) {
      node.nodeValue = translated;
    }
    translatedText.set(node, translated);
    node = walker.nextNode() as Text | null;
  }
}

function translateAttributes(root: HTMLElement, language: Language) {
  const elements = root.querySelectorAll<HTMLElement>("*");
  elements.forEach((element) => {
    if (element.closest("[data-i18n-skip='true']")) {
      return;
    }
    for (const name of attributeNames) {
      const value = element.getAttribute(name);
      if (!value) {
        continue;
      }
      const dataName = `data-i18n-original-${name}`;
      const original = element.getAttribute(dataName) ?? value;
      if (!element.hasAttribute(dataName)) {
        element.setAttribute(dataName, original);
      }
      const translated = translateText(original, language);
      if (value !== translated) {
        element.setAttribute(name, translated);
      }
    }
  });
}

function shouldSkipNode(node: Text) {
  return Boolean(node.parentElement?.closest("[data-i18n-skip='true']"));
}
