package main

import (
	"bufio"
	"fmt"
	"golang.org/x/net/html"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

var idDb map[int]string
var idDbLock sync.RWMutex
var dbFileName string = "zhihu.db"
var maxTodo int = 0x1000
var thresholdTodo int = maxTodo - 1096
var lenTodo int = 0
var maxProcessor int = 3

type record struct {
	id    int
	title string
}

func main() {
	var file *os.File
	result := make(chan record)
	todo := make(chan int, maxTodo)
	idDb = make(map[int]string)
	sem := make(chan int, maxProcessor)

	file, err := os.OpenFile(dbFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("[%s]: %s\n", dbFileName, err.Error())
		return
	}
	defer file.Close()

	totalLoaded := loadRecord(file)
	fmt.Printf("Load %d record(s) from %s\n", totalLoaded, dbFileName)

	go recorder(file, result)

	todo <- 20313419

	for {
		lenTodo = len(todo)
		id := <-todo
		//fmt.Println("get", id)
		idDbLock.RLock()
		_, ok := idDb[id]
		idDbLock.RUnlock()
		if ok {
			continue
		}
		sem <- 1
		go func() {
			url := fmt.Sprintf("http://www.zhihu.com/question/%d", id)
			processUrl(url, result, todo)
			<-sem
		}()
	}
}

func recorder(file *os.File, result chan record) {
	for {
		r := <-result
		idDbLock.Lock()
		val, ok := idDb[r.id]
		if !ok {
			// 记录新的id和title
			fmt.Printf("[%4d] %d %s\n", lenTodo, r.id, r.title)
			wbuf := fmt.Sprintln(r.id, r.title)
			_, err := file.WriteString(wbuf)
			if err != nil {
				fmt.Printf("[%d %s]: %s\n", r.id, r.title, err.Error())
			} else {
				idDb[r.id] = r.title
			}
		} else {
			// 已经有记录了
			if val != r.title {
				fmt.Printf("[%d %s]: WARNING new title %s\n", r.id, val, r.title)
			}
		}
		idDbLock.Unlock()
	}
}

func loadRecord(file *os.File) int {
	s := bufio.NewScanner(file)
	var r string
	var totalLoaded int = 0

	idDbLock.Lock()
	for s.Scan() {
		r = s.Text()
		idx := strings.Index(r, " ")
		if idx == -1 {
			fmt.Println("Invalid record:", r)
			continue
		}
		id := r[:idx]
		title := r[idx+1:]
		dec, err := strconv.Atoi(id)
		if err != nil {
			fmt.Println("Invalid record:", r)
			continue
		}
		idDb[dec] = title
		totalLoaded++
	}
	idDbLock.Unlock()

	if err := s.Err(); err != nil {
		fmt.Println(err.Error())
	}

	return totalLoaded
}

func processUrl(url string, result chan record, todo chan int) int {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("[%s]: %s\n", url, err.Error())
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
		fmt.Printf("[%s]: Not find title\n", url)
		return -1
	}
	title = strings.TrimSpace(title)
	title = strings.Replace(title, "\n", " ", -1)
	id, success := parseZhRef(url)
	if success {
		var r record
		r.id = id
		r.title = title
		result <- r
	} else {
		fmt.Printf("[%s]: Not find id\n", url)
	}

	if lenTodo > thresholdTodo {
		return 0
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Printf("[%s]: %s\n", url, err.Error())
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
					todo <- id
					//fmt.Println("added", id)
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

func parseZhRef(ref string) (int, bool) {
	var id string
	var idx int
	if strings.HasPrefix(ref, "http://www.zhihu.com/question/") {
		id = strings.TrimPrefix(ref, "http://www.zhihu.com/question/")
	} else if strings.HasPrefix(ref, "/question/") {
		id = strings.TrimPrefix(ref, "/question/")
	} else {
		return -1, false
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
	if idx = strings.Index(id, "/"); idx != -1 {
		id = id[:idx]
	}
	dec, err := strconv.Atoi(id)
	if err != nil {
		fmt.Printf("Invalid id %s in %s: %s\n", id, ref, err.Error())
		return -1, false
	}
	return dec, true
}
