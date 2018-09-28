package main

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type view struct {
	Code int
	Data struct {
		Title  string
		Videos int
		Owner  struct {
			Name string
			Mid  int
		}
		Cid   int
		Pages []struct {
			Cid      int
			Page     int
			Part     string
			Duration int
		}
	}
}

func (v *view) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("标  题: %s\n", v.Data.Title))
	buf.WriteString(fmt.Sprintf("上传者: %s (mid: %d)\n", v.Data.Owner.Name, v.Data.Owner.Mid))
	if v.Data.Videos > 1 {
		buf.WriteString("分  页:\n")
		for i, p := range v.Data.Pages {
			buf.WriteString(fmt.Sprintf("      %3d. %s - %s\n", i+1, `"`+p.Part+`"`, parseDuration(p.Duration)))
		}
	} else {
		buf.WriteString(fmt.Sprintf("长  度: %s\n", parseDuration(v.Data.Pages[0].Duration)))
	}
	return buf.String()
}

func main() {
	var aid string
	var page int
	var outFile string
	var parseBiu, infoOnly bool
	flag.StringVar(&aid, "a", "", "aid, 视频的AV号")
	flag.IntVar(&page, "p", 1, "视频分页号")
	flag.StringVar(&outFile, "o", "", "输出到指定文件")
	flag.BoolVar(&parseBiu, "b", false, "转换为Biu格式")
	flag.BoolVar(&infoOnly, "i", false, "打印视频信息并退出")
	flag.Parse()

	if aid == "" {
		flag.Usage()
		os.Exit(1)
	}
	if strings.HasPrefix(aid, "av") || strings.HasPrefix(aid, "AV") {
		aid = aid[2:]
	}
	var view view
	view, err := getView(aid)
	if err != nil || view.Code != 0 {
		exitWithErr("无法获取cid")
	}
	if infoOnly {
		fmt.Print(view.String())
		return
	}
	if page < 1 || page > view.Data.Videos {
		exitWithErr("分页号错误")
	}
	cid := strconv.Itoa(view.Data.Pages[page-1].Cid)
	resp, err := httpGet("https://api.bilibili.com/x/v1/dm/list.so?oid=" + cid)
	if err != nil {
		exitWithErr("无法获取弹幕列表")
	}
	defer resp.Body.Close()
	fr := flate.NewReader(resp.Body)
	data, _ := ioutil.ReadAll(fr)
	if outFile != "" {
		f, err := os.Create(outFile)
		if err != nil {
			exitWithErr(err.Error())
		}
		defer f.Close()
		_, err = f.Write(data)
		if err != nil {
			exitWithErr(err.Error())
		}
	} else {
		fmt.Println(string(data))
	}
}

func getView(aid string) (view view, err error) {
	resp, err := httpGet("https://api.bilibili.com/x/web-interface/view?aid=" + aid)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &view)
	return
}

func httpGet(url string) (resp *http.Response, err error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Chrome/23.3.3333.333")
	resp, err = client.Do(req)
	return
}

func exitWithErr(msg string) {
	fmt.Fprintln(os.Stderr, "错误: "+msg)
	os.Exit(1)
}

func parseDuration(dur int) string {
	second := dur % 60
	minute := dur / 60
	return fmt.Sprintf("%02d:%02d", minute, second)
}
