package main

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"encoding/xml"
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
	if parseBiu {
		if data, err = convertToBiuFormat(data); err != nil {
			exitWithErr("转换格式时发生错误: " + err.Error())
		}
	}
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
	var client http.Client
	var req *http.Request
	if req, err = http.NewRequest("GET", url, nil); err == nil {
		req.Header.Set("User-Agent", "Chrome/23.3.3333.333")
		resp, err = client.Do(req)
	}
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

func convertToBiuFormat(data []byte) (jsonData []byte, err error) {
	type xmlDanmaku struct {
		D []struct {
			P    string `xml:"p,attr"`
			Text string `xml:",chardata"`
		} `xml:"d"`
	}
	type biuDanmaku struct {
		Time  int    `json:"time"`
		Type  int    `json:"type"`
		Text  string `json:"text"`
		Style struct {
			Color string `json:"color"`
		} `json:"style"`
	}
	xmlData := new(xmlDanmaku)
	if err = xml.Unmarshal(data, xmlData); err != nil {
		return nil, err
	}
	biuData := make([]biuDanmaku, len(xmlData.D))
	for i := 0; i < len(biuData); i++ {
		p := strings.Split(xmlData.D[i].P, ",")
		timeStr := strings.Replace(p[0], ".", "", -1)
		if biuData[i].Time, err = strconv.Atoi(timeStr[:len(timeStr)-2]); err != nil {
			return nil, err
		}
		if p[1] == "1" || p[1] == "2" || p[1] == "3" {
			biuData[i].Type = 1
		} else if p[1] == "5" {
			biuData[i].Type = 2
		} else if p[1] == "4" {
			biuData[i].Type = 3
		}
		biuData[i].Text = xmlData.D[i].Text
		if color, err := strconv.Atoi(p[3]); err == nil {
			biuData[i].Style.Color = fmt.Sprintf("#%06x", color) // 颜色转换, dec -> hex
		} else {
			return nil, err
		}
	}
	return json.Marshal(biuData)
}

func dec2hex(dec string) (hex string, err error) {
	var n int
	hexArr := make([]rune, 0, 10)
	if n, err = strconv.Atoi(dec); err != nil {
		return
	}
	for {
		r := n % 16
		if r < 10 {
			hexArr = append(hexArr, rune('0'+r))
		} else {
			hexArr = append(hexArr, rune(r-10+'A'))
		}
		if n /= 16; n == 0 {
			break
		}
	}
	for i, j := 0, len(hexArr)-1; i < j; i, j = i+1, j-1 {
		hexArr[i], hexArr[j] = hexArr[j], hexArr[i]
	}
	hex = string(hexArr)
	return
}
