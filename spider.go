package main

import (
	"fmt"
	"golang.org/x/net/html"
	"net/http"
	"os"
	"strings"
)

type video struct {
	id    string
	title string
}

func main() {
	result := make(chan video)
	todo := make(chan string)
	pageDB := make(map[string]string)
	var next string

	go recorder(result, pageDB)
	initilize(result, todo, pageDB)
	for {
		next = <-todo
		_, ok := pageDB[next]
		if !ok {
			pageDB[next] = "NA"
			go worker(next, result, todo)
		}
	}
}

func initilize(result chan video, todo chan string, pageDB map[string]string) {
	resp, err := http.Get("http://www.youku.com/")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			attr := n.Attr
			for _, a := range attr {
				if a.Key == "href" && strings.HasPrefix(a.Val, "http://v.youku.com/v_show/id_") {
					id := parseYkRef(a.Val)
					_, ok := pageDB[id]
					if !ok {
						pageDB[id] = "NA"
						go worker(id, result, todo)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
}

func recorder(result chan video, pageDB map[string]string) {
	var ret video
	num := 0
	for {
		ret = <-result
		val, ok := pageDB[ret.id]
		if !ok {
			fmt.Printf("[BUG] Not in DB: %s %s\n", ret.id, ret.title)
		} else if val != "NA" {
			fmt.Printf("[BUG] already hash: %s %s(%s)\n", ret.id, val, ret.title)
		}
		pageDB[ret.id] = ret.title
		fmt.Printf("[%4d] %s\t  %s\n", num, ret.id, ret.title)
		num++
	}
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
	if idx = strings.Index(id, "_v_"); idx != -1 {
		id = id[:idx]
	}
	return id
}

func worker(id string, result chan video, todo chan string) {
	ref := fmt.Sprintf("http://v.youku.com/v_show/id_%s.html", id)
	resp, err := http.Get(ref)
	if err != nil {
		return
		//panic(err)
	}
	defer resp.Body.Close()

	title := "NA"
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
	var ret video
	ret.id = id
	ret.title = title
	result <- ret

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			attr := n.Attr
			for _, a := range attr {
				if a.Key == "href" && strings.HasPrefix(a.Val, "http://v.youku.com/v_show/id_") {
					id := parseYkRef(a.Val)
					todo <- id
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
}
