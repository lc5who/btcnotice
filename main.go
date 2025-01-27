package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"context"

	"github.com/PuerkitoBio/goquery"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/parnurzeal/gorequest"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	_ "modernc.org/sqlite"

	// "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type Announcement struct {
	Title string
	URL   string
}

var db *sql.DB
var client = lark.NewClient("cli_a7052195e97c101c", "XymdfaXB9UICNSYRfcUYeeTMZtb2Ck3A")

func initDB() error {
	var err error
	db, err = sql.Open("sqlite", "./data.db")
	if err != nil {
		return err
	}

	// Create announcements table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS announcements (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL UNIQUE,
		source TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func isAnnouncementSent(title string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM announcements WHERE title = ?", title).Scan(&count)
	return count > 0, err
}

func saveAnnouncement(title, source string) error {
	_, err := db.Exec("INSERT INTO announcements (title, source) VALUES (?, ?)", title, source)
	return err
}

func initSelenium() (*selenium.Service, selenium.WebDriver, error) {
	log.Printf("开始初始化 Selenium...")
	// 设置 Selenium 服务的配置
	opts := []selenium.ServiceOption{}
	service, err := selenium.NewChromeDriverService("chromedriver", 9515, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("启动 ChromeDriver 失败: %v", err)
	}
	log.Printf("ChromeDriver 服务已启动")

	// 设置 Chrome 的配置
	caps := selenium.Capabilities{
		"browserName": "chrome",
	}
	chromeCaps := chrome.Capabilities{
		Args: []string{
			// "--headless",
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-gpu",
			"--window-size=1920,1080",
			"--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
	}
	caps.AddChrome(chromeCaps)

	// 连接到 WebDriver
	log.Printf("正在连接到 WebDriver...")
	driver, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", 9515))
	if err != nil {
		service.Stop()
		return nil, nil, fmt.Errorf("连接 WebDriver 失败: %v", err)
	}
	log.Printf("WebDriver 连接成功")

	return service, driver, nil
}

func main() {
	// Initialize database
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("程序开始运行")
	discordWebhookURL := "https://discord.com/api/webhooks/1238374149169086485/KB6dyjyVgNOD6pBNfoYrj4P0L4d8y_G9WChXJC17sbsmmRASS14mPPYSJrCvedUSsx9A"

	// proxy := "http://127.0.0.1:7897"
	// request := gorequest.New().Proxy(proxy)
	request := gorequest.New()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	// checkAnnouncementsBiAn()
	// checkAnnouncementsOKx(discordWebhookURL, request)
	// go func() {
	// 	for {
	// 		checkAnnouncementsBiAn()
	// 		time.Sleep(5 * time.Second)
	// 	}
	// }()
	go func() {
		for {
			checkAnnouncementsOKx(discordWebhookURL, request)
			time.Sleep(5 * time.Second)
		}
	}()
	for range ticker.C {
		checkAnnouncementsBiAn()
	}

}

func checkAnnouncementsBiAn() {
	log.Printf("开始检查币安公告...")
	// 初始化 Selenium
	service, driver, err := initSelenium()
	if err != nil {
		log.Printf("初始化 Selenium 失败: %v", err)
		return
	}
	defer service.Stop()
	defer driver.Quit()

	// 访问币安公告页面
	url := "https://www.binance.com/zh-CN/support/announcement/%E6%95%B0%E5%AD%97%E8%B4%A7%E5%B8%81%E5%8F%8A%E4%BA%A4%E6%98%93%E5%AF%B9%E4%B8%8A%E6%96%B0?c=48&navId=48"
	log.Printf("正在访问页面: %s", url)
	if err := driver.Get(url); err != nil {
		log.Printf("访问页面失败: %v", err)
		return
	}

	// 等待页面加载
	log.Printf("等待页面加载...")
	time.Sleep(10 * time.Second)

	// 获取页面源码
	log.Printf("获取页面源码...")
	pageSource, err := driver.PageSource()
	if err != nil {
		log.Printf("获取页面源码失败: %v", err)
		return
	}

	// 保存页面源码到文件（用于调试）
	// file, err := os.OpenFile("1.html", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	// if err != nil {
	// 	log.Printf("创建文件失败: %v", err)
	// 	return
	// }
	// defer file.Close()
	// file.WriteString(pageSource)

	// 使用正则表达式匹配公告标题
	re := regexp.MustCompile(`<h3 class="typography-body1-1">(币安将上市.*?)</h3>`)
	matches := re.FindAllStringSubmatch(pageSource, -1)

	foundAnnouncements := false
	for _, match := range matches {
		if len(match) >= 2 {
			title := match[1]
			log.Printf("找到标题: %s", title)
			foundAnnouncements = true

			sent, err := isAnnouncementSent(title)
			if err != nil {
				log.Printf("检查公告状态失败: %v", err)
				continue
			}
			if !sent {
				log.Printf("发现新公告: %s", title)
				sendToDiscordV1(title)
				if err := saveAnnouncement(title, "binance"); err != nil {
					log.Printf("保存公告失败: %v", err)
				}
			}
		}
	}

	if !foundAnnouncements {
		log.Printf("未找到任何公告内容")
	}
}

func checkAnnouncementsOKx(webhookURL string, client *gorequest.SuperAgent) {
	url := "https://www.okx.com/zh-hans/help/section/announcements-new-listings"

	resp, body, errs := client.Get(url).End()
	if len(errs) > 0 {
		log.Printf("获取页面失败: %v", errs)
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		log.Printf("解析HTML失败: %v\n原始内容: %s", err, body[:500])
		return
	}

	doc.Find(".index_title__iTmos").Each(func(i int, s *goquery.Selection) {
		title := s.Text()
		if strings.Contains(title, "欧易关于上线") {
			sent, err := isAnnouncementSent(title)
			if err != nil {
				log.Printf("检查公告状态失败: %v", err)
			}
			if !sent {
				fmt.Println(title)
				sendToDiscordV1(title)
				if err := saveAnnouncement(title, "okx"); err != nil {
					log.Printf("保存公告失败: %v", err)
				}
			}
		}
	})

}

func sendToDiscordV1(title string) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(`chat_id`).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(`oc_7c6cf0f56f15d17f00671a6c73ea9bba`).
			MsgType(`text`).
			Content(fmt.Sprintf(`{"text":"%s"}`, title)).
			Build()).
		Build()

	// 发起请求
	_, err := client.Im.V1.Message.Create(context.Background(), req)

	// 处理错误
	if err != nil {
		fmt.Println(err)
		return
	}
}

func sendToDiscordV2(title string) {
	content := map[string]string{
		"text": fmt.Sprintf("新上币公告:\n%s", title),
	}
	jsonContent, _ := json.Marshal(content)
	msg := map[string]string{
		"receive_id": "oc_7c6cf0f56f15d17f00671a6c73ea9bba",
		"msg_type":   "text",
		"content":    string(jsonContent),
	}
	//content := discordwebhook.Message{
	//	Username: &botName,
	//	Content:  &message,
	//}
	//
	//err := discordwebhook.SendMessage(webhookURL, content)

	_, a, err := gorequest.New().Post("https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id").Set("Content-Type", "application/json").
		Set("Authorization", "Bearer t-g1041rfGD3MWDE4KKQJJTQY6YKHRUXX6RM7ALLE5").
		SendMap(msg).End()
	log.Printf(a)
	if err != nil {
		log.Printf("发送飞书消息失败: %v", err)
		return
	}

	log.Printf("成功发送公告到飞书: %s", title)
}
