package telegram

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
)

func writeExportJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err = os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// A crash can leave a partial trailing line. Recover to the last complete record.
func repairExportJSONL(path string) (map[int]bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	seen := map[int]bool{}
	var valid int64
	for {
		line, e := reader.ReadBytes('\n')
		if len(line) > 0 {
			var row textExportRecord
			if line[len(line)-1] != '\n' || json.Unmarshal(line, &row) != nil {
				if err = f.Truncate(valid); err != nil {
					return nil, err
				}
				break
			}
			seen[row.ID] = true
			valid += int64(len(line))
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
	}
	return seen, nil
}

const exportStyle = `<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'"><style>body{max-width:1000px;margin:30px auto;padding:0 20px;font:16px/1.7 system-ui;color:#222}article{border-bottom:1px solid #ddd;padding:15px 0}pre{white-space:pre-wrap;overflow-wrap:anywhere;font:inherit}small{color:#666}a{color:#1762b0;overflow-wrap:anywhere}td,th{padding:8px;text-align:left;border-bottom:1px solid #ddd}table{border-collapse:collapse;width:100%}</style>`

var exportHeader = template.Must(template.New("header").Parse(`<!doctype html><html lang="zh-CN"><head><title>{{.Title}}</title>` + exportStyle + `</head><body><p><a href="index.html">返回导出目录</a></p><h1>{{.Title}}</h1><p>共 {{.Count}} 条；按从新到旧排列。媒体本体未下载，文字说明已保留。</p>{{if not .Complete}}<p>此群组导出尚未完成：{{.Error}}</p>{{end}}`))
var exportMessage = template.Must(template.New("message").Parse(`<article id="m{{.ID}}"><small>{{.Date.Format "2006-01-02 15:04:05 MST"}} · {{.Sender}} · #{{.ID}}{{if .Edited}} · 已编辑{{end}}</small><pre>{{.Text}}</pre>{{if .Media}}<small>附件类型：{{.Media}}（未下载本体）</small>{{end}}{{range .Links}}<p><a href="{{.URL}}" rel="noreferrer">{{.Text}}</a></p>{{end}}{{if .SourceURL}}<a href="{{.SourceURL}}" rel="noreferrer">查看 Telegram 原始消息</a>{{else}}<small>该群组没有可用的原始消息链接</small>{{end}}{{if .ReplyTo}} · <a href="#m{{.ReplyTo}}">回复消息 #{{.ReplyTo}}</a>{{end}}</article>`))

func renderExportGroup(out string, g *TextExportGroup) error {
	f, err := os.OpenFile(filepath.Join(out, g.Key+".html"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	writer := bufio.NewWriter(f)
	if err = exportHeader.Execute(writer, g); err != nil {
		return err
	}
	source, err := os.Open(filepath.Join(out, g.Key+".jsonl"))
	if err != nil {
		return err
	}
	defer source.Close()
	decoder := json.NewDecoder(source)
	g.Restricted = 0
	for {
		var row textExportRecord
		if err = decoder.Decode(&row); err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if row.Kind == "protected" { g.Restricted++ }
		if err = exportMessage.Execute(writer, row); err != nil {
			return err
		}
	}
	if _, err = writer.WriteString("</body></html>"); err != nil {
		return err
	}
	return writer.Flush()
}

func renderExportIndex(out string, m TextExportManifest) error {
	tmpl := template.Must(template.New("index").Parse(`<!doctype html><html lang="zh-CN"><head><title>Telegram 群组消息导出</title>` + exportStyle + `</head><body><h1>Telegram 群组消息导出</h1><p>范围：{{.Start}} 至 {{.End}}；时区：Asia/Singapore。</p><p>只包含群组和频道，不含私人聊天。未下载媒体本体。{{if not .Complete}}部分群组导出未完成，请查看下表和 manifest.json。{{end}}</p><table><tr><th>群组／频道</th><th>消息数</th><th>状态</th><th>数据</th></tr>{{range .Groups}}<tr><td><a href="{{.Key}}.html">{{.Title}}</a></td><td>{{.Count}}</td><td>{{if .Complete}}已完成{{else}}未完成：{{.Error}}{{end}}</td><td><a href="{{.Key}}.jsonl">JSONL</a></td></tr>{{end}}</table><p>来源被删除、账号无权访问或 Telegram 未提供的历史无法导出。内容中的链接不会在打开本页时自动访问。</p></body></html>`))
	f, err := os.OpenFile(filepath.Join(out, "index.html"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, m)
}

func zipTextExport(out string) error {
	path := filepath.Join(out, "telegram-text-export.zip")
	f, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	z := zip.NewWriter(f)
	entries, err := os.ReadDir(out)
	if err != nil {
		z.Close()
		f.Close()
		return err
	}
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || (ext != ".html" && ext != ".jsonl" && entry.Name() != "manifest.json") {
			continue
		}
		source, e := os.Open(filepath.Join(out, entry.Name()))
		if e != nil {
			z.Close()
			f.Close()
			return e
		}
		w, e := z.Create(entry.Name())
		if e == nil {
			_, e = io.Copy(w, source)
		}
		source.Close()
		if e != nil {
			z.Close()
			f.Close()
			return e
		}
	}
	if err = z.Close(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(path+".tmp", path); err != nil {
		return fmt.Errorf("finalize_export_zip: %w", err)
	}
	return nil
}
