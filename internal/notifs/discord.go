package notifs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"time"
)

type Provider interface {
	SendMessage(title string, desc string, msgKey string, msgValue string)
}

type Footer struct {
	Text    string `json:"text"`
	IconUrl string `json:"icon_url,omitempty"`
}

type Embed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Url         string `json:"url,omitempty"`
	Color       int    `json:"color"`
	Footer      Footer `json:"footer,omitempty"`
}

type Discord struct {
	webhook string
	Embed   []*Embed `json:"embeds"`
}

func NewDiscordInfo(webhook string) Provider {
	d := &Discord{}
	d.webhook = webhook
	footer := Footer{Text: "Eagle Eye"}

	embed := &Embed{
		Color:  3447003,
		Footer: footer,
	}

	d.Embed = []*Embed{embed}
	return d
}

func (d *Discord) SendMessage(title string, desc string, msgKey string, msgValue string) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)

	jsonPart, err := writer.CreateFormField("payload_json")
	if err != nil {
		log.Println(err)
		return
	}

	d.Embed[0].Title = fmt.Sprintf(":telescope: %s", title)
	d.Embed[0].Description = fmt.Sprintf(":cyclone: **%s**", desc)

	jsonPayload, err := json.Marshal(d)
	if err != nil {
		log.Println(err)
		return
	}
	jsonPart.Write(jsonPayload)

	filePart, err := writer.CreateFormFile("file", fmt.Sprintf("%s.txt", msgKey))
	if err != nil {
		log.Println(err)
		return
	}

	_, err = filePart.Write([]byte(msgValue))
	if err != nil {
		log.Println(err)
		return
	}
	writer.Close()

	d.sendEmbedReq(writer, buffer)
}

func (d *Discord) sendEmbedReq(writer *multipart.Writer, data *bytes.Buffer) {
	resp, err := http.Post(d.webhook, writer.FormDataContentType(), data)
	if err != nil {
		log.Printf("[!] Error sending discord request: %v\nurl: %s", err, d.webhook)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		log.Printf("[!] Weird status code from discord: %s\n", resp.Status)
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[!] Response: %s\n", string(body))
	}
	

	time.Sleep(500 * time.Millisecond)
}
