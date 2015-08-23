package main

import (
	"fmt"
	"golang.org/x/net/html"
	"net/http"
	"strings"
)

type video struct {
	id    string
	title string
}

func main() {
	resp, err := http.Get("http://www.youku.com/")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	pageDB := make(map[string]string)

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			attr := n.Attr
			for _, a := range attr {
				if a.Key == "href" && strings.HasPrefix(a.Val, "http://v.youku.com/v_show/id_") {
					id := parseYkRef(a.Val)
					_, ok := pageDB[id]
					if !ok {
						recordID(id, pageDB)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	fmt.Println("Done!")
}

func parseYkRef(ref string) (id string) {
	id = strings.TrimPrefix(ref, "http://v.youku.com/v_show/id_")
	var idx int
	if idx = strings.Index(id, ".html"); idx != -1 {
		id = id[:idx]
	}
	if idx = strings.Index(id, "=="); idx != -1 {
		id = id[:idx]
	}
	return id
}

func recordID(id string, pageDB map[string]string) {
	ref := fmt.Sprintf("http://v.youku.com/v_show/id_%s.html", id)
	resp, err := http.Get(ref)
	if err != nil {
		return
		//panic(err)
	}
	defer resp.Body.Close()

	title := "N/A"
	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		t := z.Token()
		if t.Type == html.StartTagToken && t.Data == "title" {
			tt = z.Next()
			t = z.Token()
			if t.Type == html.TextToken {
				title = t.Data
				title = strings.TrimSuffix(title, "—优酷网，视频高清在线观看")
				break
			}
		}
	}

	fmt.Printf("%s\t   %s\n", id, title)
	pageDB[id] = title
}
