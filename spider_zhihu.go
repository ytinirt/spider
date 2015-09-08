package main

import (
	"bufio"
	"fmt"
	"golang.org/x/net/html"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var idDb map[int]string
var idDbLock sync.RWMutex
var dbFileName string = "zhihu.db"
var maxTodo int = 0x4000
var thresholdTodo int = maxTodo - 1384
var lenTodo int = 0
var maxProcessor int = 3
var useRand bool = true
var maxValidId int = -1
var tmpId int = -1
var maxGenRandId int = 1000

type record struct {
	id    int
	title string
}

func main() {
	var id int
	var file *os.File
	result := make(chan record)
	todo := make(chan int, maxTodo)
	idDb = make(map[int]string)
	sem := make(chan int, maxProcessor)

	//args := os.Args[1:]
	//if args != nil {
	//	val, err := strconv.Atoi(args[0])
	//	if err == nil && val > 0 {
	//		useRand = false
	//		maxValidId = val
	//		tmpId = maxValidId
	//	}
	//}

	file, err := os.OpenFile(dbFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("[%s]: %s\n", dbFileName, err.Error())
		return
	}
	defer file.Close()

	totalLoaded, _ := loadRecord(file)
	fmt.Printf("Load %d record(s) from %s\n", totalLoaded, dbFileName)

	start := genStartId()
	fmt.Println("Start id:", start)

	todo <- start

	go recorder(file, result)

	for {
		lenTodo = len(todo)

		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(1 * time.Second)
			timeout <- true
		}()

		select {
		case id = <-todo:
		case <-timeout:
			go genRandId(todo)
			continue
		}

		//fmt.Println("get", id)
		idDbLock.RLock()
		_, ok := idDb[id]
		idDbLock.RUnlock()
		if ok {
			continue
		}
		sem <- 1
		go func() {
			nid := id
			url := fmt.Sprintf("http://www.zhihu.com/question/%d", nid)
			processUrl(url, result, todo)
			<-sem
		}()
	}
}

func genStartId() int {
	var id int

	rand.Seed(time.Now().UnixNano())

	for {
		if useRand {
			id = rand.Intn(15734175) + 19550225
		} else {
			id = tmpId
			tmpId--
		}
		if id <= 0 {
			panic("id <= 0")
		}

		idDbLock.RLock()
		_, ok := idDb[id]
		idDbLock.RUnlock()

		if ok {
			continue
		}

		url := fmt.Sprintf("http://www.zhihu.com/question/%d", id)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		if resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		// fmt.Printf("[%d]: %s\n", resp.StatusCode, url)
		resp.Body.Close()
	}

	return id
}

func genRandId(todo chan int) {
	var id int
	count := maxGenRandId

	rand.Seed(time.Now().UnixNano())

	for count > 0 {
		id = rand.Intn(16000000) + 19550000

		idDbLock.RLock()
		_, ok := idDb[id]
		idDbLock.RUnlock()

		if ok {
			continue
		}

		url := fmt.Sprintf("http://www.zhihu.com/question/%d", id)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		statusCode := resp.StatusCode
		resp.Body.Close()
		if statusCode == 200 {
			count--
			todo <- id
		}
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

func loadRecord(file *os.File) (totalLoaded int, lastId int) {
	s := bufio.NewScanner(file)
	var r string
	totalLoaded = 0
	lastId = -1

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
		val, ok := idDb[dec]
		if ok {
			fmt.Println("Already exist:", dec, val)
		} else {
			idDb[dec] = title
		}
		lastId = dec
		totalLoaded++
	}
	idDbLock.Unlock()

	if err := s.Err(); err != nil {
		fmt.Println(err.Error())
	}

	return totalLoaded, lastId
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
