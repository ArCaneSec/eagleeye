package notifs

import (
	"fmt"
	"strings"
)

type Notify interface {
	NewAssetNotif(target string, domain string, assets []string)
	ErrNotif(err error)
	NewDnsNotif(assets []string)
	NewHttpNotif(hosts []string)
}

type Notif struct {
	Webhook  string
	provider Provider
}

func NewNotif(webhook string) Notify {
	return &Notif{
		webhook,
		NewDiscordInfo(webhook),
	}
}

func (n Notif) NewAssetNotif(target string, domain string, assets []string) {
	// var l []string
	// for i := 0; i < 100; i++ {
	// 	l = append(l, fmt.Sprint(i))

	// }
	// strAssets := strings.Join(l, "\n")
	strAssets := strings.Join(assets, "\n")

	n.provider.SendMessage("New Subdomains",
		fmt.Sprintf("%d new subdomains found for %s", len(assets), target),
		fmt.Sprintf("domain: %s", domain),
		strAssets,
	)
}

func (n Notif) ErrNotif(err error) {
	fmt.Println(err)
}

func (n Notif) NewDnsNotif(assets []string) {
	strAssets := strings.Join(assets, "\n")

	n.provider.SendMessage("New A Records",
		fmt.Sprintf("%d new A records found.", len(assets)),
		"Assets:",
		strAssets,
	)
}

func (n Notif) NewHttpNotif(hosts []string) {
	strAssets := strings.Join(hosts, "\n")

	n.provider.SendMessage("New Http Services",
		fmt.Sprintf("%d new http services found.", len(hosts)),
		"Hosts:",
		strAssets,
	)
}