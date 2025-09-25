package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	serverchan_sdk "github.com/easychen/serverchan-sdk-golang"
)

type Airdrop struct {
	Token           string      `json:"token"`
	Name            string      `json:"name"`
	Date            string      `json:"date"`
	Time            string      `json:"time"`
	Points          interface{} `json:"points"` // 修复：改为 int 类型
	Amount          string      `json:"amount"`
	Type            string      `json:"type"`
	Phase           int         `json:"phase"`
	Status          string      `json:"status"`
	SystemTimestamp int64       `json:"system_timestamp"`
	Completed       bool        `json:"completed"`
	ContractAddress string      `json:"contract_address"`
	ChainID         string      `json:"chain_id"`
}

type ApiResponse struct {
	Airdrops []Airdrop `json:"airdrops"`
}

// 配置结构体
type Config struct {
	SendKeys []string `json:"sendkeys"`
	Interval int      `json:"interval"`
	FiterTge bool     `json:"fiterTge"`
}

// 读取配置文件
func loadConfig() (*Config, error) {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// 发送到Server酱
func sendToServerChan(msg string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	title := "今日空投播报"
	for _, sendkey := range cfg.SendKeys {
		resp, err := serverchan_sdk.ScSend(sendkey, title, msg, nil)
		if err != nil {
			fmt.Printf("推送Server酱失败: %v\n", err)
		} else {
			fmt.Println("Server酱响应:", resp)
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func getAirdrop() *ApiResponse {
	url := "https://alpha123.uk/api/data?t=1751632712002&fresh=1"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println(err)
		return nil
	}

	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	req.Header.Set("referer", "https://alpha123.uk/")
	req.Header.Set("user-agent", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return nil
	}

	var apiResp ApiResponse
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		log.Println(err)
		return nil
	}

	for i, item := range apiResp.Airdrops {
		if item.Phase == 2 && item.Date != "" && item.Time != "" {
			// 时间加18小时
			layout := "2006-01-02 15:04"
			parsed, err := time.Parse(layout, item.Date+" "+item.Time)
			if err == nil {
				parsed = parsed.Add(18 * time.Hour)
				item.Date = parsed.Format("2006-01-02")
				item.Time = parsed.Format("15:04")
			}
			apiResp.Airdrops[i] = item
		}
	}

	return &apiResp
}

// 获取token单价
// 批量获取所有价格
func fetchAllTokenPrices() (map[string]float64, error) {
	url := "https://alpha123.uk/api/price/?batch=today"
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	req.Header.Set("referer", "https://alpha123.uk/")
	req.Header.Set("user-agent", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Prices  map[string]struct {
			Token    string  `json:"token"`
			Price    float64 `json:"price"`
			DexPrice float64 `json:"dex_price"`
		} `json:"prices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("batch price fetch failed")
	}

	prices := make(map[string]float64)
	for token, data := range result.Prices {
		// 优先使用dex_price，如果为0则使用price
		if data.DexPrice > 0 {
			prices[token] = data.DexPrice
		} else {
			prices[token] = data.Price
		}
	}
	return prices, nil
}

func getSendMsgAndSnapshot() (string, string) {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// 一次性获取所有价格
	allPrices, err := fetchAllTokenPrices()
	if err != nil {
		fmt.Printf("获取价格失败: %v\n", err)
		allPrices = make(map[string]float64)
	}

	apiResp := getAirdrop()
	if apiResp == nil {
		// 处理错误，比如直接 return
		return "", ""
	}
	msg := "| 项目 | 时间 | 积分 | 数量 | 阶段 | 价格(USD) |\n|---|---|---|---|---|---|\n"
	snapshot := ""
	isEmpty := true
	for i, item := range apiResp.Airdrops {
		var amount int
		if item.Amount != "" {
			amount, err = strconv.Atoi(item.Amount)
			if err != nil {
				fmt.Printf("转换数量失败: %v\n", err)
				amount = 0
			}
		} else {
			amount = 0
		}
		// 比较日期是否是今天
		if item.Date != time.Now().Format("2006-01-02") {
			continue
		}

		if cfg.FiterTge && item.Type == "tge" {
			fmt.Printf("过滤TGE: %+v\n", item)
			continue
		}

		// 改善价格获取错误处理
		price := allPrices[item.Token] // 直接从map获取
		fmt.Printf("单价: %v\n", price)

		var points int
		switch v := item.Points.(type) {
		case string:
			points, _ = strconv.Atoi(v)
		case float64:
			points = int(v)
		case int:
			points = v
		default:
			points = 0
		}

		msg += fmt.Sprintf("| %s(%s) | %s %s | %d | %s | %d | %.2f |\n", // 修复：%d 替换 %s
			item.Token, item.Name, item.Date, item.Time, points, item.Amount, item.Phase, price*float64(amount))
		snapshot += fmt.Sprintf("%s|%s|%s|%s|%s|%d\n",
			item.Token, item.Name, item.Date, item.Time, item.Amount, item.Phase)
		apiResp.Airdrops[i] = item
		isEmpty = false
	}
	if isEmpty {
		return "", ""
	}
	return msg, snapshot
}

func hashMsg(msg string) string {
	h := md5.New()
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	var lastHash string
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	interval := time.Duration(cfg.Interval) * time.Minute

	for {
		msg, snapshot := getSendMsgAndSnapshot()
		currentHash := hashMsg(snapshot)

		if currentHash != lastHash && msg != "" {
			fmt.Println("检测到空投信息变化，推送通知...")
			fmt.Println(msg)
			if err := sendToServerChan(msg); err != nil {
				fmt.Println("推送Server酱失败:", err)
			}
			lastHash = currentHash
		} else {
			fmt.Println("无变化，无需推送。")
		}
		time.Sleep(interval)
	}
}
