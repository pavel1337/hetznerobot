package main

import (
	"encoding/json"
	"flag"
    "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type Config struct {
	AuthorizedIDs []int64 `json:"authorized_ids"`
	BotApi        string  `json:"telegram_bot_api"`
	User        string `json:"user"`
	Password    string `json:"password"`
}

type Server struct {
	Server struct {
		ServerIP     string   `json:"server_ip"`
		ServerNumber int      `json:"server_number"`
		ServerName   string   `json:"server_name"`
		Product      string   `json:"product"`
		Dc           string   `json:"dc"`
		Traffic      string   `json:"traffic"`
		Flatrate     bool     `json:"flatrate"`
		Status       string   `json:"status"`
		Throttled    bool     `json:"throttled"`
		Cancelled    bool     `json:"cancelled"`
		PaidUntil    string   `json:"paid_until"`
		IP           []string `json:"ip"`
		Subnet       []struct {
			IP   string `json:"ip"`
			Mask string `json:"mask"`
		} `json:"subnet"`
	} `json:"server"`
}

type ResetOptions struct {
	StatusCode int64
	Status string
	Reset struct {
		ServerIP        string   `json:"server_ip"`
		ServerNumber    int      `json:"server_number"`
		Type            []string `json:"type"`
		OperatingStatus string   `json:"operating_status"`
	} `json:"reset"`
}


type Reset struct {
	StatusCode int64
	Status string
	Reset struct {
		ServerIP string `json:"server_ip"`
		Type     string `json:"type"`
	} `json:"reset"`
}

func main() {
	path, debug := parseFlags()

	c, err := parseConfig(*path)
	if err != nil {
		log.Fatalf("ERROR: %s", err)
		return
	}

    bot, err := tgbotapi.NewBotAPI(c.BotApi)
    if err != nil {
		log.Fatalf("ERROR: %s", err)
    }
    if *debug {
    	bot.Debug = true
    }

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates, err := bot.GetUpdatesChan(u)
    if err != nil {
		log.Fatalf("ERROR: %s", err)
    }
    for update := range updates {
        if update.Message == nil { // ignore any non-Message updates
            continue
        }

        if !IsAuthorizedPerson(c.AuthorizedIDs, update.Message.Chat.ID) {
        	continue
        }

        // Create a new MessageConfig. We don't have text yet,
        // so we should leave it empty.
        msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

        // Extract the command from the Message.
        switch update.Message.Command() {
        case "help":
        	msg.Text = "Available commands:\n/list - to list servers\n/reset IP - to issue reset"
        case "list":
        	servers, err := ListServers(c)
        	if err != nil {
        		log.Fatalf("ERROR: %s", err)
        	}
        	for i := range servers {
        		msg.Text += servers[i].Server.ServerIP + " " +  servers[i].Server.ServerName + "\n"
        	}
        case "reset":
        	IP := update.Message.CommandArguments()
        	if IP == "" {
        		msg.Text = "Please provide a valid IP"
        	} else {
	        	reset, err := GetResetServer(c, IP)
	        	if err != nil {
	        		log.Fatalf("ERROR: %s", err)
	        	}
	        	if reset.Reset.ServerIP == IP {
		        	msg.Text = "Available reset types:\n"
		        	for i := range reset.Reset.Type {
		        		msg.Text += reset.Reset.Type[i] + " "
		        	}
		        	msg.Text += "\nTo reset server with sw type send:\n\n/reset_sure "+reset.Reset.ServerIP+" sw"
	        	}
        	}
        case "reset_sure":
        	args := update.Message.CommandArguments()
        	if args == "" {
        		msg.Text = "Please provide a valid arguments"
        	} else {
        		reset, err := PostResetServer(c, args)
	        	if err != nil {
	        		log.Fatalf("ERROR: %s", err)
	        	}
        		msg.Text = reset.Status
        	}
        default:
            msg.Text = "Click /help if dont know what to do"
        }

        if _, err := bot.Send(msg); err != nil {
			log.Fatalf("ERROR: %s", err)
        }
    }
}

func IsAuthorizedPerson(ids []int64, chatid int64) bool {
	for i := range ids {
		if ids[i] == chatid {
			return true
		}
	}
	return false
}

func PostResetServer(c Config, args string) (Reset, error) {
	var reset Reset
	argsSplitted := strings.Split(args, " ")
	resetType := "type=" + argsSplitted[1]
	body := strings.NewReader(resetType)
	postUrl := "https://robot-ws.your-server.de/reset/" + argsSplitted[0]

	req, err := http.NewRequest("POST", postUrl, body)
	if err != nil {
		return reset, err
	}

	req.SetBasicAuth(c.User, c.Password)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return reset, err
	}
	reset.Status = resp.Status
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&reset)
	if err != nil {
		return reset, err
	}

	return reset, nil
}

func GetResetServer(c Config, IP string) (ResetOptions, error) {
	var reset ResetOptions
	getUrl := "https://robot-ws.your-server.de/reset/" + IP
	req, err := http.NewRequest("GET", getUrl, nil)
	if err != nil {
		return reset, err
	}
	req.SetBasicAuth(c.User, c.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return reset, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&reset)
	if err != nil {
		return reset, err
	}

	return reset, nil
}


func ListServers(c Config) ([]Server, error) {
	var servers []Server
	req, err := http.NewRequest("GET", "https://robot-ws.your-server.de/server", nil)
	if err != nil {
		return servers, err
	}
	req.SetBasicAuth(c.User, c.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return servers, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&servers)
	if err != nil {
		return servers, err
	}

	return servers, nil
}

func parseFlags() (*string, *bool) {
	configPathHelpInfo := " path to config file"
	configPath := flag.String("c", "", configPathHelpInfo)
	debug := flag.Bool("D", false, "debug the bot")
	flag.Parse()
	return configPath, debug
}

func parseConfig(p string) (Config, error) {
	var c Config
	rawConfig, err := ioutil.ReadFile(p)
	if err != nil {
		flag.Usage()
		return c, err
	}
	err = yaml.Unmarshal(rawConfig, &c)
	if err != nil {
		return c, err
	}
	return c, nil
}
