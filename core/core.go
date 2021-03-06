package core

import (
	"CYS2/stack"
	"errors"
	"fmt"
	"github.com/go-resty/resty"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var JpgUrls = stack.New() //用来保存获取到的作者套图页面的URL
var RealJpgs = stack.New()
var ThreadSync sync.WaitGroup

var client = resty.New()

func openURL() (req *resty.Request, err error) {
	req = resty.R()
	req.SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/70.0.3538.110 Safari/537.36")
	req.SetHeader("Connection", "keep-alive")
	req.SetHeader("Accept", "*/*")
	req.SetHeader("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7,zh-TW;q=0.6,ja;q=0.5")
	return
}

/***
传入网址
@return 	result 获取到的页面数据
				nextUrl 该网页下一页的网址
				err 	错误
*/
func GetMainPage(url string) (result, nextUrl string, err error) {
	fmt.Printf("正在打开页面：%s\n", url)
	client.R().SetHeader("Referer", "http://www.ciyo.cn/")
	client.R().SetHeader("Host", "www.ciyo.cn")
	resp, err := client.R().Get(url)
	/*req, err :=openURL()
	req.SetHeader("Referer", "http://www.ciyo.cn/")
	req.SetHeader("Host", "www.ciyo.cn")
	resp, err := req.Get(url)*/
	if err != nil {
		return
	}
	if resp.StatusCode() != 200 {
		log.Fatal(string(resp.Body()))
		fmt.Println("GetMainPage server error")
		return
	}

	result = string(resp.Body())

	next := resp.Header().Get("CY-NextUrl")
	nextUrl = "http://www.ciyo.cn" + next
	err = nil
	return
}

/**
把主页面数据中的子页面网址提取到JpgUrls中保存
@pram 主页面数据
*/
func GetChildUrl(mainPage string) (err error) {
	compile := regexp.MustCompile(`<a href="(?s:(.*?))">`)
	if compile == nil {
		return errors.New("函数GetChildPage() regexp compile error")
	}
	childUrls := compile.FindAllStringSubmatch(mainPage, -1) //过滤网页内容，提取子页面网址，-1代表过滤全部
	for _, data := range childUrls {
		childUrl := data[1]
		if childUrl == "" {
			return errors.New("未获取到mainPage中URL地址")
		}
		JpgUrls.Push("http://www.ciyo.cn" + childUrl)
	}
	return nil
}

/**
获取作者套图页面数据
*/
func GetJpgPage() (err error) { //多线程

	defer ThreadSync.Done()
	var author string //保存作者名字
	s := JpgUrls.Pop()
	client.R().SetHeader("Upgrade-Insecure-Requests", "1")
	client.R().SetHeader("Host", "www.ciyo.cn")
	client.R().SetHeader("Referer", "http://www.ciyo.cn/")
	resp, err := client.R().Get(s)

	/*req, _ := openURL()
	req.SetHeader("Upgrade-Insecure-Requests", "1")
	req.SetHeader("Referer", "http://www.ciyo.cn/")
	req.SetHeader("Host", "www.ciyo.cn")
	resp, err := req.Get(s)*/
	fmt.Println("正在打开网页")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if resp.StatusCode() != 200 {
		log.Println(string(resp.Body()))
		log.Println("GetJpgPage server error")
		return
	}

	body := resp.Body()
	res := string(body)

	//提取作者的名字
	name, e := regexp.Compile(`-name">(?s:(.*?))</span>`)
	if e != nil {
		e = errors.New("func GetJpgPage0 regexp compile error")
		return
	}
	realName := name.FindAllStringSubmatch(res, 1)

	for _, data := range realName {
		author = data[1]
		break
	}
	fmt.Println("作者名字提取到了：" + author)

	//提取图片地址
	compile, e := regexp.Compile(`class="images"(?s:(.*?))</ul>`)
	if e != nil {
		e = errors.New("func GetJpgPage1 regexp compile error")
		return
	}
	jpgs := compile.FindAllStringSubmatch(res, 1)
	var remix string
	for _, data := range jpgs {
		remix = data[1]
		break
	}
	compile2, e := regexp.Compile(`src="(?s:(.*?))" />`)
	if e != nil {
		e = errors.New("func GetJpgPage2 regexp compile error")
		return
	}
	jpgs2 := compile2.FindAllStringSubmatch(remix, -1)
	var mix string
	t := 0
	for _, data := range jpgs2 {
		mix = data[1]
		if mix == "" {
			strings.Trim(mix, " ")
			log.Println("未提取取到realJpg地址：\n" + res)
			return errors.New("未提取取到realJpg地址：\n" + res)
		}
		t++
		log.Println("提取到realJpg地址" + mix)
		RealJpgs.Push(mix)
	}

	//调用获取真实的一张图片的接口

	for i := 0; i < t; i++ {
		ThreadSync.Add(1)
		go GetRealJpg(author)
	}
	return
}

func GetRealJpg(author string) { //多线程
	time.Sleep(time.Second)
	defer ThreadSync.Done()
	s := RealJpgs.Pop()
	client.R().SetHeader("Upgrade-Insecure-Requests", "1")
	client.R().SetHeader("Host", "qn.ciyocon.com")
	client.R().SetHeader("Cache-Control", "max-age=0")
	resp, err := client.R().Get(s)

	/*req, _ := openURL()
	req.SetHeader("Upgrade-Insecure-Requests", "1")
	req.SetHeader("Cache-Control", "max-age=0")
	req.SetHeader("Host", "qn.ciyocon.com")
	resp, err := req.Get(s)*/
	if err != nil {
		log.Println("RealJpgs,len()=", strconv.Itoa(RealJpgs.Len()), "错误问题", err.Error())
		return
	}

	//defer resp.Body.Close()
	if resp.StatusCode() != 200 {
		log.Println("GetRealJpg server error code:", resp.StatusCode(), s, string(resp.Body()))
		return
	}
	WriteFile(author, resp.Body())
}

func WriteFile(author string, stream []byte) (err error) {
	_, ee := os.Stat("./Download")
	if ee != nil {
		os.Mkdir("./Download", os.ModePerm) //创建Download文件夹
	}
	_, ee = os.Stat("./Download/" + author)
	if ee != nil {
		os.Mkdir("./Download/"+author, os.ModePerm) //以作者名字创建目录
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	filename := strconv.Itoa(r.Intn(999999))
	e := ioutil.WriteFile("./Download/"+author+"/"+filename+".jpg", stream, 0666)
	if e != nil {
		fmt.Println(e.Error())
	}
	return
}
