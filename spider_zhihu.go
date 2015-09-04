package main

import (
	"bufio"
	"fmt"
	"golang.org/x/net/html"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var idDB map[string]string
var file *os.File

func main() {
	idDB = make(map[string]string)
	f, err := os.OpenFile("zhihu.db", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer f.Close()
	file = f

	loadRecord()

	process("http://www.zhihu.com/question/19783289")
}

func loadRecord() {
	s := bufio.NewScanner(file)
	var record string

	for s.Scan() {
		record = s.Text()
		idx := strings.Index(record, " ")
		if idx == -1 {
			fmt.Println("Invalid record:", record)
			continue
		}
		id := record[:idx]
		title := record[idx+1:]
		if _, err := strconv.Atoi(id); err != nil {
			fmt.Println("Invalid record:", record)
			continue
		}
		idDB[id] = title
	}
	if err := s.Err(); err != nil {
		fmt.Println(err.Error())
	}
}

func process(url string) int {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err.Error())
		return -1
	}
	defer resp.Body.Close()

	z := html.NewTokenizer(resp.Body)
	title := "N/A"
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
				break
			}
		}
	}
	if title == "N/A" {
		fmt.Println("Not find title")
		return -1
	}
	title = strings.TrimSpace(title)
	fmt.Println(url, title)
	id, success := parseZhRef(url)
	if success {
		record := fmt.Sprintln(id, title)
		_, err := file.WriteString(record)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			err = file.Sync()
			if err != nil {
				fmt.Println(err.Error())
			}
			idDB[id] = title
		}
	} else {
		fmt.Println("Not find id in: ", url)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return -1
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			attr := n.Attr
			for _, a := range attr {
				if a.Key == "href" {
					id, success := parseZhRef(a.Val)
					if !success {
						break
					}
					processZhId(id)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return 0
}

func parseZhRef(ref string) (string, bool) {
	var id string
	var idx int
	if strings.HasPrefix(ref, "http://www.zhihu.com/question/") {
		id = strings.TrimPrefix(ref, "http://www.zhihu.com/question/")
	} else if strings.HasPrefix(ref, "/question/") {
		id = strings.TrimPrefix(ref, "/question/")
	} else {
		return "", false
	}

	if idx = strings.Index(id, "?"); idx != -1 {
		id = id[:idx]
	}
	if idx = strings.Index(id, "#"); idx != -1 {
		id = id[:idx]
	}
	if idx = strings.Index(id, "/answer"); idx != -1 {
		id = id[:idx]
	}
	return id, true
}

func processZhId(id string) {
	_, ok := idDB[id]
	if !ok {
		url := fmt.Sprintf("http://www.zhihu.com/question/%s", id)
		process(url)
	}
}
