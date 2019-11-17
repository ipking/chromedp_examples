package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"io/ioutil"
	"log"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Signature struct {
	Code int `json:"code"`
	Data struct {
		Accessid  string `json:"accessid"`
		Enable    bool   `json:"enable"`
		Expire    string `json:"expire"`
		Host      string `json:"host"`
		Policy    string `json:"policy"`
		Signature string `json:"signature"`
	} `json:"data"`
	Encode string `json:"encode"`
	Msg    string `json:"msg"`
	Time   int    `json:"time"`
}

func main() {
	dir, err := ioutil.TempDir("", "chromedp-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.UserDataDir(dir),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// also set up a custom logger
	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// create a timeout
	taskCtx, cancel = context.WithTimeout(taskCtx, 5*time.Second)
	defer cancel()

	// listen network event
	var wg sync.WaitGroup

	flag := false
	chromedp.ListenTarget(taskCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventResponseReceived:
			if strings.Index(ev.Response.URL ,"ossDataService") != -1 && strings.Index(ev.Response.URL ,"appName=pc_tusou") != -1 && flag == false{
				flag = true
				wg.Add(1)
				go func(RequestID network.RequestID,URL string) {
					defer wg.Add(-1)
					log.Println("执行开始")
					c := chromedp.FromContext(taskCtx)
					body, _ := network.GetResponseBody(RequestID).Do(cdp.WithExecutor(taskCtx, c.Target))
					var str string = string(body[:])
					cP, _ := url.Parse(URL)

					//开始上传文件
					str_slice := strings.Split(cP.RawQuery,"callback=")
					new_str  := str_slice[1]

					json_str := strings.Replace(str,new_str+"(","",1)
					json_str = strings.Replace(json_str,");","",1)
					//json str 转struct
					var config Signature
					json.Unmarshal([]byte(json_str), &config);
					path, _ := os.Getwd()
					path += "/1688/1.jpg"

					name := "1.jpg"
					time_str := strconv.FormatInt(time.Now().Unix(),10)
					key := "cbuimgsearch/" + CreateRandomString(10)+time_str+".jpg"

					policy := config.Data.Policy
					OSSAccessKeyId := config.Data.Accessid
					signature := config.Data.Signature

					extraParams := map[string]string{
						"name": name,
						"key": key,
						"policy": policy,
						"OSSAccessKeyId": OSSAccessKeyId,
						"success_action_status": "200",
						"callback": "",
						"signature": signature,
					}
					request, _ := newFileUploadRequest("https://cbusearch.oss-cn-shanghai.aliyuncs.com/", extraParams, "file", path)
					client := &http.Client{}
					//提交请求
					resp,_ := client.Do(request)
					defer resp.Body.Close()

					if resp.StatusCode == 200{
						//说明上传成功
						target_url := "https://s.1688.com/youyuan/index.htm?tab=imageSearch&imageType=oss&imageAddress="+key+"&spm=a260k.dacugeneral.search.0"
						log.Println(target_url)
					}
					log.Println("执行结束")
				}(ev.RequestID,ev.Response.URL)
			}
		}
		// other needed network Event
	})

	chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			c := chromedp.FromContext(ctx)
			url := []string{"*.jpg", "*.png", "*.gif", "*.css", "*.woff", "*.ttf", "*.woff2"}
			_ = network.SetBlockedURLS(url).Do(cdp.WithExecutor(ctx, c.Target))
			return nil
		}),
		chromedp.Navigate(`https://www.1688.com/?spm=b26110380.8880418.0.d5.5e12adccWWDX8V`),
		chromedp.WaitVisible(`body`, chromedp.BySearch),
	)

	wg.Wait()
}


// Creates a new file upload http request with optional extra params
func newFileUploadRequest(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
	file_data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, val := range params {
		writer.WriteField(key, val)
	}
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	part.Write(file_data)
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", uri, body)
	request.Header.Add("Content-Type", writer.FormDataContentType())

	return request, err
}

func CreateRandomString(len int) string  {
	var container string
	var str = "ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678"
	b := bytes.NewBufferString(str)
	length := b.Len()
	bigInt := big.NewInt(int64(length))
	for i := 0;i < len ;i++  {
		randomInt,_ := rand.Int(rand.Reader,bigInt)
		container += string(str[randomInt.Int64()])
	}
	return container
}