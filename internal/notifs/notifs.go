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
	NucleiResultsNotif(string)
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
	strAssets := strings.Join(assets, "\n")

	n.provider.SendMessage("New Subdomains",
		fmt.Sprintf("%d new subdomains found for %s", len(assets), target),
		domain,
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
		"dns-records",
		strAssets,
	)
}

func (n Notif) NewHttpNotif(hosts []string) {
	strAssets := strings.Join(hosts, "\n")

	n.provider.SendMessage("New Http Services",
		fmt.Sprintf("%d new http services found.", len(hosts)),
		"http-services",
		strAssets,
	)
}

func (n Notif) NucleiResultsNotif(results string) {
	n.provider.SendMessage("Nuclei Results", "Nuclei results with newly templates.", "nuclei-results", results)
}