package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

type Provider interface {
	SendMessage(jobName string, desc string, msgKey string, msgValue string)
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type Footer struct {
	Text    string `json:"text"`
	IconUrl string `json:"icon_url,omitempty"`
}

type Embed struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Url         string  `json:"url,omitempty"`
	Color       int     `json:"color"`
	Fields      []Field `json:"fields"`
	Footer      Footer  `json:"footer,omitempty"`
}

type Discord struct {
	webhook string
	Embed   []Embed `json:"embeds"`
	Content string  `json:"content,omitempty"`
}

func NewInfo() Provider {
	d := &Discord{}
	d.webhook = os.Getenv("DISCORD_WEBHOOK")
	footer := Footer{Text: "Eagle Eye"}

	embed := Embed{
		Color:  3447003,
		Footer: footer,
	}

	d.Embed = []Embed{embed}

	return d
}

func (d *Discord) SendMessage(jobName string, desc string, msgKey string, msgValue string) {
	if len(msgValue) >= 1024 {

		half := len(msgValue) / 2
		firstHalfLastWord := msgValue[half]

		for string(firstHalfLastWord) != "\n" {
			half++
			firstHalfLastWord = msgValue[half]
		}

		firstHalf := msgValue[:half]
		secondHalf := msgValue[half:]

		var wg *sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()

			newDis := NewInfo()
			newDis.SendMessage(jobName, desc, msgKey, firstHalf)
		}()

		go func() {
			defer wg.Done()

			newDis := NewInfo()
			newDis.SendMessage(jobName, desc, msgKey, secondHalf)
		}()

		wg.Wait()
		return
	}

	d.Embed[0].Title = fmt.Sprintf(":telescope: %s", jobName)
	d.Embed[0].Description = fmt.Sprintf(":cyclone: **%s**", desc)

	field := Field{fmt.Sprintf(":dart: **%s**", msgKey), fmt.Sprintf("```\n%s\n```", msgValue), false}
	fmt.Println(msgValue)

	d.Embed[0].Fields = []Field{field}
	d.sendEmbedReq()
}

func (d *Discord) sendEmbedReq() {
	messageBytes, err := json.Marshal(d)
	if err != nil {
		fmt.Printf("[!] Error marshalling message: %v\n", err)
	}

	resp, err := http.Post(d.webhook, "application/json", bytes.NewBuffer(messageBytes))
	if err != nil {
		fmt.Printf("[!] Error sending request: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		fmt.Printf("[!] Weird status code from discord: %s\n", resp.Status)
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Response: %s\n", string(body))
	}
}
