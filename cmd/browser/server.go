package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"

	"github.com/Xelckis/icmpbrowser/internal/sender"
	"golang.org/x/net/icmp"
)

var tmpl *template.Template = template.Must(template.ParseFiles("templates/index.html"))

type siteData struct {
	Url    string
	Err    bool
	ErrMsg string
	Html   template.HTML
}

func main() {

	ipFlag := flag.String("ip", "::2", "Endereço IPv6 local do servidor web")
	proxyIPFlag := flag.String("proxy", "::1", "Endereço IPv6 do proxy de destino")
	flag.Parse()

	localAddr := *ipFlag
	c, err := icmp.ListenPacket("ip6:58", localAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	helloHandler := func(w http.ResponseWriter, r *http.Request) {
		t := tmpl
		queryParams := r.URL.Query()

		targetUrl := queryParams.Get("url")
		if targetUrl == "" {
			t.ExecuteTemplate(w, "index.html", siteData{Err: true, ErrMsg: "url is empty", Html: ""})
			return
		}

		htmlString, err := sender.SendMessage(c, targetUrl, *proxyIPFlag)
		if err != nil {
			t.ExecuteTemplate(w, "index.html", siteData{Err: true, ErrMsg: err.Error()})
			return
		}

		baseURL, parseErr := url.Parse(targetUrl)
		if parseErr == nil {
			re := regexp.MustCompile(`href=["']([^"']+)["']`)
			htmlString = re.ReplaceAllStringFunc(htmlString, func(match string) string {
				submatch := re.FindStringSubmatch(match)
				if len(submatch) < 2 {
					return match
				}
				linkOriginal := submatch[1]

				if linkOriginal != "" && linkOriginal[0] == '#' {
					return match
				}

				parsedLink, err := url.Parse(linkOriginal)
				if err != nil {
					return match
				}

				linkAbsoluto := baseURL.ResolveReference(parsedLink).String()

				return fmt.Sprintf(`href="/?url=%s"`, url.QueryEscape(linkAbsoluto))
			})
		}

		err = t.ExecuteTemplate(w, "index.html", siteData{targetUrl, false, "", template.HTML(htmlString)})
		if err != nil {
			t.ExecuteTemplate(w, "index.html", siteData{Err: true, ErrMsg: fmt.Sprintf("Error getting the url html: %s", err)})
			return
		}

		log.Println("Terminou de ler")

	}

	http.HandleFunc("/", helloHandler)
	fmt.Println("Server is running. Enter localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
