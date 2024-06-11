package notifs

import (
	"fmt"
	"strings"
)

type Notify interface {
	NewAssetNotif(target string, domain string, assets []string)
	ErrNotif(details string, err error)
	NewDnsNotif(assets []string)
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

	n.provider.SendMessage("Subdomain Enumeration",
		fmt.Sprintf("%d new subdomains found for %s", len(assets), target),
		fmt.Sprintf("domain: %s", domain),
		strAssets,
	)
}

func (n Notif) ErrNotif(detail string, err error) {
	fmt.Println(detail)
	fmt.Println(err)
}

func (n Notif) NewDnsNotif(assets []string) {
	strAssets := strings.Join(assets, "\n")

	n.provider.SendMessage("Resolve New Subs",
		fmt.Sprintf("%d new A records found.", len(assets)),
		"Assets:",
		strAssets,
	)
}
