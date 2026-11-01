package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// Upload is generated in memory; it never refers to a local filesystem path.
type Upload struct {
	Filename    string
	ContentType string
	Data        []byte
}

func requestBody(payload any) ([]byte, string, error) {
	fields, ok := payload.(map[string]any)
	hasFile := false
	if ok {
		for _, value := range fields {
			if _, ok := value.(Upload); ok {
				hasFile = true
				break
			}
		}
	}
	if !hasFile {
		body, err := json.Marshal(payload)
		return body, "application/json", err
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for name, value := range fields {
		if file, ok := value.(Upload); ok {
			if file.Filename == "" || strings.ContainsAny(file.Filename, "\r\n") || len(file.Data) > 1<<20 {
				return nil, "", fmt.Errorf("bot_document_invalid")
			}
			if _, _, err := mime.ParseMediaType(file.ContentType); err != nil {
				return nil, "", fmt.Errorf("bot_document_invalid")
			}
			header := textproto.MIMEHeader{}
			header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": name, "filename": file.Filename}))
			header.Set("Content-Type", file.ContentType)
			part, err := writer.CreatePart(header)
			if err != nil {
				return nil, "", fmt.Errorf("bot_document_invalid")
			}
			if _, err = part.Write(file.Data); err != nil {
				return nil, "", err
			}
		} else {
			text, ok := value.(string)
			if !ok {
				data, err := json.Marshal(value)
				if err != nil {
					return nil, "", err
				}
				text = string(data)
			}
			if err := writer.WriteField(name, text); err != nil {
				return nil, "", err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

// Call supports private activity interactions through the existing Bot transport.
// Network errors intentionally omit the request URL, which contains the token.
func (s *Service) Call(ctx context.Context, token, method string, payload any, out any) error {
	body, contentType, e := requestBody(payload)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", s.apiBaseURL+"/bot"+token+"/"+method, bytes.NewReader(body))
	if e != nil {
		return fmt.Errorf("bot_request_invalid")
	}
	req.Header.Set("Content-Type", contentType)
	resp, e := s.client.Do(req)
	if e != nil {
		return fmt.Errorf("bot_network_failed")
	}
	defer resp.Body.Close()
	var envelope struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&envelope); e != nil {
		return fmt.Errorf("bot_response_invalid")
	}
	if resp.StatusCode >= 300 || !envelope.OK {
		return fmt.Errorf("bot_api_status_%d", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(envelope.Result, out)
	}
	return nil
}
