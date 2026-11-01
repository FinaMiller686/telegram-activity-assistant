"use client";
import { useCallback, useEffect, useState } from "react";
import { DashboardPage, Surface } from "@/components/dashboard-page";
import { Button, Input, Field } from "@/components/ui";
import { CalendarButton } from "@/components/calendar-button";

type Event = {id:number; nameEn:string; nameZh:string; organizer:string; summary:string; category:string;date:string;dateText:string;timeText:string;deadlineText:string;start:string|null;end:string|null;location:string;room:string;floor:string;registrationUrl:string;deadline:string|null;deadlineDate:string;eligibility:string;dressCode:string;registrationRequired:boolean|null;warnings:string[];status:string;withdrawn:boolean;conflicts:number[];sources:{chatId:number;messageId:number;title:string;link:string;deleted:boolean}[]};
type Config={aiEnabled:boolean;collectSince:string|null;digestTime:string;reminderHours:number;retentionDays:number;ownerId:number};
type DeliveryRecord={key:string;kind:string;content:string;state:string;sentAt:string|null;createdAt:string;retryAt:string;messageId:number};
type DeliveryReport={queuedMessages:number;failedJobs:number;exhaustedJobs:number;pendingDeliveries:number;retryingDeliveries:number;unconfirmedDeliveries:number;monitoredGroups:number;staleGroups:number;oldestSync:string|null;botReady:boolean;recent:DeliveryRecord[]};
const deliveryStates:Record<string,string>={sent:"Telegram 已接收",unchanged:"内容未变，未重复发送",superseded:"已被新版替代",inactive:"已过期或已取消推送",pending:"待发送",retrying:"发送失败，等待重试",unconfirmed:"发送结果未确认，将重试"};
const deliveryKinds:Record<string,string>={card:"活动卡片",deadline:"截止提醒",digest:"每日摘要",digestpart:"摘要片段"};
const runtimeNames:Record<string,string>={worker:"处理任务",ai:"AI 处理",history:"历史回补",delivery:"Bot 投递",bot:"Bot 交互",source_audit:"来源核对",extraction:"信息提取"};
const runtimeCodes:Record<string,string>={ok:"正常",storage_encryption_required:"等待加密存储",database_or_processing_error:"数据库或处理错误",bot_delivery_failed:"Bot 投递失败，将重试",bot_delivery_retrying:"部分推送等待重试",model_request_failed:"模型请求失败",calendar_delivery_failed:"日历文件发送失败，请稍后重试。",poll_failed_check_token_or_webhook:"Bot 接收失败，请检查 Token 或 webhook",gap_exceeds_30_days_manual_backfill_required:"间隔超过 30 天，需要手动回补",backfill_start_failed:"回补启动失败",backfill_failed:"回补失败",telegram_request_failed:"Telegram 请求失败",database_error:"数据库错误",unexpected_response:"Telegram 响应异常",store_failed:"保存失败"};
const categories=["校园活动","学术讲座","金融与商业活动","职业发展与实习","比赛与工作坊","学生福利","其他重要通知"];
const statuses:Record<string,string>={new:"新活动",going:"我要参加",later:"稍后决定",ignored:"不感兴趣",registered:"已完成报名",attended:"已参加"};
async function call<T>(path:string,method="GET",body?:unknown):Promise<T>{const r=await fetch(path,{method,credentials:"include",headers:{"Content-Type":"application/json"},body:body===undefined?undefined:JSON.stringify(body)});if(r.status===401){window.location.href="/login";throw Error("请先登录")}if(!r.ok){throw Error("操作未完成，请检查配置与系统状态。")}return r.json()}
function when(s:string|null){return s?new Intl.DateTimeFormat("zh-CN",{timeZone:"Asia/Singapore",year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",hour12:false}).format(new Date(s)):"未知"}
function hasInfo(s?:string|null){return !!s?.trim()&&!/^(未知|unknown|原文未注明|未提供|n\/a|null|tba|tbc|待定)$/i.test(s.trim())}
function safeURL(s:string){try{const u=new URL(s);return ["https:","http:"].includes(u.protocol)&&!u.username&&!u.password}catch{return false}}
export function ActivitiesPanel(){
 const [items,setItems]=useState<Event[]>([]),[cfg,setCfg]=useState<Config|null>(null),[logs,setLogs]=useState<{name:string;code:string;updatedAt:string}[]>([]),[failed,setFailed]=useState(0),[protectedStorage,setProtectedStorage]=useState(false),[digests,setDigests]=useState<{key:string;content:string;sentAt:string|null}[]>([]);
 const [delivery,setDelivery]=useState<DeliveryReport|null>(null);
 const [cfgDirty,setCfgDirty]=useState(false);
 const [q,setQ]=useState(""),[category,setCategory]=useState(""),[status,setStatus]=useState(""),[from,setFrom]=useState(""),[to,setTo]=useState(""),[message,setMessage]=useState(""),[busy,setBusy]=useState(false),[confirm,setConfirm]=useState("");
 const refresh=useCallback(async()=>{const params=new URLSearchParams({q,category,status,from,to});try{const [events,config,runtime,history]=await Promise.all([call<{items:Event[]}>("/api/activities?"+params),call<Config>("/api/activity/config"),call<{items:typeof logs;failedJobs:number;storageProtected:boolean;delivery:DeliveryReport}>("/api/activity/runtime"),call<{items:typeof digests}>("/api/activity/digests")]);setItems(events.items);setCfg(current=>cfgDirty?current:config);setLogs(runtime.items);setFailed(runtime.failedJobs);setDelivery(runtime.delivery);setProtectedStorage(runtime.storageProtected);setDigests(history.items)}catch(e){setMessage((e as Error).message)}},[q,category,status,from,to,cfgDirty]);
 useEffect(()=>{void refresh();const timer=setInterval(()=>void refresh(),15000);return()=>clearInterval(timer)},[refresh]);
 async function run(fn:()=>Promise<unknown>){setBusy(true);setMessage("");try{await fn();setMessage("已保存");await refresh()}catch(e){setMessage((e as Error).message)}finally{setBusy(false)}}
 return <DashboardPage title="活动助手" description="按新加坡时间整理活动、报名与个人参加清单。" actions={<Button onClick={()=>void refresh()}>刷新</Button>}>
 {message&&<p role="status">{message}</p>}
 {!protectedStorage&&<Surface title="助手加密存储不可用"><p>尚未确认数据存储保护，账号连接、群组监听和 Bot 推送已暂停。请按对应系统的安装说明检查配置。</p></Surface>}
 <Surface title="活动列表"><div className="activity-filters">
 <Input aria-label="关键词" placeholder="关键词" value={q} onChange={e=>setQ(e.target.value)}/>
 <select aria-label="类别" value={category} onChange={e=>setCategory(e.target.value)}><option value="">全部类别</option>{categories.map(c=><option key={c} value={c}>{c}</option>)}</select>
 <select aria-label="状态" value={status} onChange={e=>setStatus(e.target.value)}><option value="">全部状态</option><option value="selected">待参加与已报名</option>{Object.entries(statuses).map(([s,label])=><option key={s} value={s}>{label}</option>)}</select>
 <Field label="开始日期"><Input type="date" value={from} onChange={e=>setFrom(e.target.value)}/></Field><Field label="结束日期"><Input type="date" value={to} onChange={e=>setTo(e.target.value)}/></Field>
 <Button onClick={()=>{const d=new Date();const fmt=(x:Date)=>new Intl.DateTimeFormat("en-CA",{timeZone:"Asia/Singapore"}).format(x);setFrom(fmt(d));d.setDate(d.getDate()+7);setTo(fmt(d))}}>未来 7 天</Button></div>
 {!items.length&&<p>暂无符合条件的活动。请先在群组页面开启监控。</p>}
 <div className="activity-grid">{items.map(v=><article className="activity-card" key={v.id}>
 <span>{v.category} · {statuses[v.status]}</span><h2>{v.nameZh||v.nameEn||"未命名活动"}</h2>{v.nameZh&&v.nameEn&&<h3>{v.nameEn}</h3>}<p>{v.summary}</p>
 <dl>
 {(hasInfo(v.date)||hasInfo(v.dateText))&&<><dt>活动日期</dt><dd>{v.date||v.dateText}</dd></>}
 {(v.start||hasInfo(v.timeText)||v.end)&&<><dt>时间</dt><dd>{v.start?when(v.start):v.timeText}{v.end?` — ${when(v.end)}`:""}</dd></>}
 {[v.location,v.floor,v.room].some(hasInfo)&&<><dt>地点</dt><dd>{[v.location,v.floor,v.room].filter(hasInfo).join(" ")}</dd></>}
 {(v.deadline||hasInfo(v.deadlineDate)||hasInfo(v.deadlineText))&&<><dt>报名截止</dt><dd>{v.deadline?when(v.deadline):v.deadlineDate||v.deadlineText}</dd></>}
 {hasInfo(v.organizer)&&<><dt>主办方</dt><dd>{v.organizer}</dd></>}
 {hasInfo(v.eligibility)&&<><dt>参与资格</dt><dd>{v.eligibility}</dd></>}
 {hasInfo(v.dressCode)&&<><dt>着装要求</dt><dd>{v.dressCode}</dd></>}
 {typeof v.registrationRequired==="boolean"&&<><dt>需要报名</dt><dd>{v.registrationRequired?"是":"否"}</dd></>}
 </dl>

 {!!v.conflicts.length&&<p className="activity-warning">这两场活动存在时间冲突。{v.conflicts.map(id=>` #${id}`).join("")}</p>}{v.withdrawn&&<p className="activity-warning">来源已删除或撤回，请核对。</p>}
 <details><summary>来源</summary>{v.sources.map(s=><p key={`${s.chatId}:${s.messageId}`}>{s.title} · 消息编号 {s.messageId} {s.deleted?"（已删除）":""} {safeURL(s.link)&&<a href={s.link} target="_blank" rel="noreferrer">查看原始消息</a>}</p>)}{v.warnings?.map((warning,i)=><p key={i}>{warning}</p>)}</details>
 <div className="activity-actions">{Object.entries(statuses).filter(([s])=>s!=="new").map(([s,label])=><Button disabled={busy} key={s} onClick={()=>void run(async()=>{await call(`/api/activities/${v.id}/status`,"PUT",{status:s});if(s==="going"){setMessage(safeURL(v.registrationUrl)?"已加入待参加清单，请使用卡片中的报名链接。":"已加入待参加清单，报名链接未知，请查看原始消息。")}})}>{label}</Button>)}</div>
 <CalendarButton event={v}/>
 </article>)}</div></Surface>
 {cfg&&<Surface title="处理与提醒设置" description="开启 AI 后，已监听群组的新文字和图片说明会发送至配置的模型；图片文件不会上传。关闭后使用本机规则。">
 <div className="activity-filters"><label><input type="checkbox" checked={cfg.aiEnabled} onChange={e=>{setCfgDirty(true);setCfg({...cfg,aiEnabled:e.target.checked})}}/>启用 AI</label>
 <Field label="本人 Telegram 用户 ID"><Input type="number" value={cfg.ownerId||""} onChange={e=>{setCfgDirty(true);setCfg({...cfg,ownerId:Number(e.target.value)})}}/></Field><Field label="每日摘要时间"><Input type="time" value={cfg.digestTime} onChange={e=>{setCfgDirty(true);setCfg({...cfg,digestTime:e.target.value})}}/></Field><Field label="截止提醒提前小时"><Input type="number" min={1} max={168} value={cfg.reminderHours} onChange={e=>{setCfgDirty(true);setCfg({...cfg,reminderHours:Number(e.target.value)})}}/></Field><Field label="原始消息保留天数"><Input type="number" min={1} max={365} value={cfg.retentionDays} onChange={e=>{setCfgDirty(true);setCfg({...cfg,retentionDays:Number(e.target.value)})}}/></Field><Button disabled={busy} onClick={()=>void run(async()=>{await call("/api/activity/config","PUT",cfg);setCfgDirty(false)})}>保存设置</Button></div>
 {cfg.collectSince&&<p><span>消息收集起点：</span>{when(cfg.collectSince)}</p>}
 <p>模型和 Bot 在系统配置中设置。Bot 仅限本人操作。</p>
 </Surface>}
 <Surface title="每日摘要历史">{digests.length?digests.map(d=><details key={d.key}><summary>{d.key.replace("digest:","")} · {d.sentAt?"已推送":"待推送"}</summary><pre style={{whiteSpace:"pre-wrap"}}>{d.content}</pre></details>):<p>每日摘要将在设定时间生成。</p>}</Surface>
 <Surface title="推送记录">
 {delivery&&<><p>{delivery.botReady?"私人 Bot 已配置":"私人 Bot 尚未就绪，请检查配置"}</p>
 <p><span>等待提取：</span>{delivery.queuedMessages} · <span>提取失败：</span>{failed} · <span>需手动重试：</span>{delivery.exhaustedJobs}</p>
 <p><span>待推送：</span>{delivery.pendingDeliveries} · <span>发送重试：</span>{delivery.retryingDeliveries} · <span>结果未确认：</span>{delivery.unconfirmedDeliveries}</p>
 <p><span>监听群组：</span>{delivery.monitoredGroups} · <span>补拉超过 45 分钟未完成：</span>{delivery.staleGroups}</p>
 {delivery.oldestSync&&<p><span>各群最近补拉的最早完成时间：</span>{when(delivery.oldestSync)}</p>}
 <p>统计只覆盖已保存的文字；纯图片通知暂不识别，AI 也可能漏判。已接收表示 Telegram 确认收到了推送，不代表手机已读。</p>
 {delivery.recent.map(d=><details key={d.key}><summary>{deliveryKinds[d.kind]||d.kind} · {deliveryStates[d.state]||d.state} · {d.content.split("\n")[0].slice(0,80)}</summary>
 <p>{when(d.sentAt||d.createdAt)}{d.messageId>0&&<span> · Telegram #{d.messageId}</span>}</p>
 {(d.state==="retrying"||d.state==="unconfirmed")&&<p><span>下次重试：</span>{when(d.retryAt)}</p>}
 <pre style={{whiteSpace:"pre-wrap"}}>{d.content}</pre></details>)}</>}
 </Surface>
 <Surface title="系统状态"><Button disabled={busy} onClick={()=>void run(()=>call("/api/activity/retry","POST",{}))}>重试失败提取</Button>{logs.map(l=><p key={l.name}>{runtimeNames[l.name]||l.name} · {runtimeCodes[l.code]||l.code} · {when(l.updatedAt)}</p>)}</Surface>
 <Surface title="删除历史数据" description="将暂停所有群组监控，并删除消息、活动、状态、摘要和本地媒体。登录凭据保留。"><Field label="输入 DELETE_HISTORY 确认"><Input value={confirm} onChange={e=>setConfirm(e.target.value)}/></Field><Button disabled={busy||confirm!=="DELETE_HISTORY"} onClick={()=>void run(()=>call("/api/activity/history","DELETE",{confirm}))}>删除历史数据</Button></Surface>
 </DashboardPage>
}
