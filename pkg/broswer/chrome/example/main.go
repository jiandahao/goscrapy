package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/jiandahao/goscrapy/pkg/broswer/chrome"
)

var chromedriverPath = flag.String("chromedriver", "/usr/local/chromedriver", "sepcified chromedriver installed path")

func main() {
	flag.Parse()

	broswer, err := chrome.NewBroswer(chrome.Config{
		ChromeDriverPath:  *chromedriverPath,
		Port:              9515,
		Headless:          false,
		UserAgent:         "Mozilla/5.0 (Windows; U; Windows NT 6.1; zh-CN) AppleWebKit/533+ (KHTML, like Gecko)",
		DownloadDirectory: "/dev/null",
	})

	if err != nil {
		fmt.Println(err)
		return
	}

	if err := broswer.Open(); err != nil {
		fmt.Println(err)
		return
	}

	if err := broswer.Get("https://www.baidu.com"); err != nil {
		fmt.Println(err)
		return
	}

	time.Sleep(time.Second * 10)

	if err := broswer.Close(); err != nil {
		fmt.Printf("failed to close browser: %v", err)
	}

	for i := 0; i < 5; i++ {
		broswer.Open()
		time.Sleep(time.Second)
		broswer.Close()
		time.Sleep(time.Second * 2)
	}
}
