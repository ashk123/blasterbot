// vim: set ts=5 sw=4 tw=99 noet:
//
// Blaster (C) Copyright 2014 AlliedModders LLC
// Licensed under the GNU General Public License, version 3 or higher.
// See LICENSE.txt for more details.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	batch "github.com/alliedmodders/blaster/batch"
	valve "github.com/alliedmodders/blaster/valve"
	"github.com/joho/godotenv"
	"golang.org/x/net/proxy"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	sOutputLock   sync.Mutex
	sOutputBuffer io.Writer
	sOutputFormat string
	sResultMap    map[int]string = make(map[int]string)
	sNumServers   int64
)

type ErrorObject struct {
	Ip    string `json:"ip"`
	Error string `json:"error"`
}

type ServerObject struct {
	Address string `json:"ip"`
	// LocalAddress string `json:"local_ip,omitempty"`
	// Protocol     uint8  `json:"protocol"`
	Name    string `json:"name"`
	MapName string `json:"map"`
	// Folder       string `json:"folder"`
	Game       string `json:"game"`
	Players    uint8  `json:"players"`
	MaxPlayers uint8  `json:"max_players"`
	// Bots         uint8  `json:"bots"`
	// Type         string `json:"type"`
	// Os           string `json:"os"`
	// Visibility   string `json:"visibility"`
	// Vac          bool   `json:"vac"`

	// Only available from The Ship.
	// Ship *valve.TheShipInfo `json:"theship,omitempty"`

	// Only available on Source.
	// AppId       valve.AppId `json:"appid,omitempty"`
	// GameVersion string      `json:"game_version,omitempty"`
	// Port        uint16      `json:"port,omitempty"`
	// SteamId     string      `json:"steamid,omitempty"`
	// GameMode    string      `json:"game_mode,omitempty"`
	// GameId      string      `json:"gameid,omitempty"`
	// SpecTvPort  uint16      `json:"spectv_port,omitempty"`
	// SpecTvName  string      `json:"spectv_name,omitempty"`

	// Only available on Half-Life 1.
	Mod *valve.ModInfo `json:"mod,omitempty"`

	Rules map[string]string `json:"rules"`
}

// Arguments used ffor blaster
type Config struct {
	flag_name    string
	flag_appids  string
	flag_appid   int
	flag_master  string
	flag_j       int // number of concurrent requests
	flag_timeout time.Duration
	flag_format  string
	flag_outfile string
	flag_norules bool
}

func (p *Config) Usage() string {
	return fmt.Sprint(
		"Usage: /search <flag_name> <flag_appid|?> <flag_master|?> <flag_j|?> <flag_norules|?>",
	)
}

const (
	BOT_ORG_NAME    string = "Blaster"
	NOT_ENOUGH_ARGS string = "Not Enough Args! Give me some information for search ..."
	// NICE            string = "NICE"
	NOT_CORRECT_ARGS string = "You didn't give me the correct arguments"
)

func greetings() string {
	return fmt.Sprintf(
		"Welcome to the %s bot!\nThis is the alternatie version of the Blaster program for the telegram\nIf you have any question use msg to tell us",
		BOT_ORG_NAME,
	)
}

func StrToBool(cond string) bool {
	return cond == "true"
}

func isEmpty(data string) string {
	if data == "" {
		return ""
	}
	return ""
}

func Parse(line string) (Config, error) {
	spl_data := strings.Split(line, " ")
	// fmt.Println(spl_data)
	if len(spl_data) <= 1 {
		return Config{}, errors.New(NOT_ENOUGH_ARGS)
	}
	// var config Config = Config{}
	// config.flag_name = spl_data[0] // save the first element inside the Config.g_name (the game name)
	// appid_number_org, err := strconv.Atoi(spl_data[1])
	// if err != nil {
	// 	return Config{}, errors.New(NOT_CORRECT_ARGS)
	// }
	// con_number, err := strconv.Atoi(spl_data[3])
	// if err != nil {
	// 	return Config{}, errors.New(NOT_CORRECT_ARGS)
	// }
	// flag_master_value := isEmpty(spl_data[2])
	return Config{
		flag_name:   spl_data[1], // save the first element inside the Config.g_name (the game name)
		flag_appids: "",
		flag_appid:  0,
		// flag_master:  spl_data[2],
		flag_master:  valve.MasterServer,
		flag_format:  "lines",
		flag_outfile: "",
		flag_j:       30,
		flag_norules: false,
	}, nil
}

func buildClientWithProxy(addr string) (*http.Client, error) {
	if addr != "" {
		dialer, err := proxy.SOCKS5("tcp", addr, nil, proxy.Direct)
		if err != nil {
			return nil, err
		}

		// Patch client transport
		httpTransport := &http.Transport{Dial: dialer.Dial}
		hc := &http.Client{Transport: httpTransport}

		return hc, nil
	}

	return nil, nil // use default
}
func addJson(hostAndPort string, obj interface{}, bot *tb.Bot, sender *tb.Message) {
	// fmt.Println(obj.(*ServerObject).Name)
	// Don't send error results

	//fmt.Println(sNumServers)
	if _, err := obj.(*ErrorObject); err {
		log.Println("ERROR:", obj.(*ErrorObject).Error)
		return
	}

	if bot == nil || sender == nil {
		return
	}

	buf, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	log.Println("1 server requested succsessfully")
	sResultServers := obj.(*ServerObject).Name + "(" + obj.(*ServerObject).MapName + ")" + " - " + " === " + obj.(*ServerObject).Game + " === " + obj.(*ServerObject).Address + " + " + fmt.Sprintf(
		"%d",
		sender.ID,
	) + "\n"
	if _, found := sResultMap[sender.ID]; found {
		sResultMap[sender.ID] += sResultServers
	} else {
		sResultMap[sender.ID] = sResultServers
	}
	//asd, err := bot.Send(sender.Sender, "\r"+obj.(*ServerObject).Name+"\r\n")
	if err != nil {
		log.Fatal("ERROR:", err)
	}
	//fmt.Println(asd)
	sOutputLock.Lock()
	defer sOutputLock.Unlock()
	return
	switch sOutputFormat {
	case "lines":
		if sNumServers != 0 {
			log.Println("I'm runnign this new line hehehehe")
			// sOutputBuffer.Write([]byte("\n"))
		}

		// sOutputBuffer.Write(buf)
		_, err := bot.Send(sender.Sender, "```\r"+string(buf)+"\r\n```")
		if err != nil {
			log.Fatal("ERROR:", err)
		}
		log.Println("I send a result for the user")
		sNumServers++
		return
	}

	if sNumServers != 0 {
		sOutputBuffer.Write([]byte(",\n"))
	}
	sOutputBuffer.Write([]byte("\t"))

	var indented bytes.Buffer
	json.Indent(&indented, buf, "\t", "\t")

	switch sOutputFormat {
	case "map":
		sOutputBuffer.Write([]byte(fmt.Sprintf("\"%s\": ", hostAndPort)))
	}

	indented.WriteTo(sOutputBuffer)
	sNumServers++
}

func addError(hostAndPort string, err error) {
	addJson(hostAndPort, &ErrorObject{
		Ip:    hostAndPort,
		Error: err.Error(),
	}, nil, nil)
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("ERROR: Can't read the env file for reason:", err.Error())
	}
	token := os.Getenv("BOT_TOKEN")
	fmt.Println(token)
	if token == "" {
		log.Println("Set token via environment\nBOT_TOKEN=<your_token>")
		return
	}

	client, err := buildClientWithProxy(os.Getenv("BOT_PROXY"))
	if err != nil {
		log.Fatal(err)
		return
	}

	b, err := tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		Client: client,
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	log.Printf("Bot started[%s]", b.Me.Username)
	b.Handle("/start", func(m *tb.Message) {
		_, err := b.Send(m.Sender, greetings())
		if err != nil {
			log.Fatal("ERROR:", err)
			return
		}
	})
	b.Handle("/search", func(m *tb.Message) {
		// waiting, err := b.Send(m.Sender, "Waiting ....")
		err := b.Notify(m.Sender, tb.Typing)
		// m.voice
		if err != nil {
			log.Print("ERROR:", err.Error())
		}
		config, err := Parse(m.Text)
		fmt.Println("Telegram Arguments Parsed")
		if err != nil {
			b.Send(m.Sender, err)
			return
		}
		sOutputFormat = config.flag_outfile
		// _, err = b.Send(m.Sender, res)
		// b.Edit(waiting, "asd")
		// b.Delete(waiting)
		// if err != nil {
		// 	log.Println("ERROR: Can't handle search handler reason:", err.Error())
		// 	return
		// }

		// flag_game := flag.String("game", "", "Game (hl1, hl2)")
		// flag_appid := flag.Int("appid", 0, "Query a single AppID")
		// flag_appids := flag.String("appids", "", "Comma-delimited list of AppIDs")
		// flag_master := flag.String("master", valve.MasterServer, "Master server address")
		// flag_j := flag.Int("j", 20, "Number of concurrent requests (more will introduce more timeouts)")
		// flag_timeout := flag.Duration("timeout", time.Second*3, "Timeout for querying servers")
		// flag_format := flag.String("format", "list", "JSON format (list, map, or lines)")
		// flag_outfile := flag.String("outfile", "", "Output to a file")
		// flag_norules := flag.Bool("norules", false, "Don't query server rules")
		// flag.Usage = func() {
		// 	fmt.Fprintf(os.Stderr, "Usage: -game or -appids\n")
		// 	flag.PrintDefaults()
		// }
		// flag.Parse()

		appids := []valve.AppId{}

		switch config.flag_format {
		case "list", "map", "lines":
			sOutputFormat = config.flag_format
		default:
			fmt.Fprintf(os.Stderr, "Unknown format type.\n")
			os.Exit(1)
		}

		if config.flag_outfile != "" {
			file, err := os.Create(config.flag_outfile)
			if err != nil {
				fmt.Fprintf(
					os.Stderr,
					"Could not open %s for writing: %s\n",
					config.flag_outfile,
					err.Error(),
				)
				os.Exit(1)
			}
			defer file.Close()

			sOutputBuffer = file
		} else {
			sOutputBuffer = os.Stdout
		}
		fmt.Printf("This is the game name: |%s|\n", config.flag_name)
		if config.flag_name != "" {
			switch config.flag_name {
			case "hl1":
				appids = append(appids, valve.HL1Apps...)
				//appids = append(appids, 70)
			case "hl2":
				appids = append(appids, valve.HL2Apps...)
			case "halflife":
				appids = append(appids, 70)

			case "css":
				appids = append(appids, 240)
			default:
				fmt.Fprintf(os.Stderr, "Unrecognized game: %s", config.flag_name)
				os.Exit(1)
			}
		}

		if config.flag_appids != "" {
			for _, part := range strings.Split(config.flag_appids, ",") {
				appid, err := strconv.Atoi(part)
				if err != nil {
					fmt.Fprintf(os.Stderr, "\"%s\" is not a valid AppID\n", part)
					os.Exit(1)
				}
				appids = append(appids, valve.AppId(appid))
			}
		}

		if config.flag_appid != 0 {
			appids = append(appids, valve.AppId(config.flag_appid))
		}

		if len(appids) == 0 {
			fmt.Fprintf(os.Stderr, "At least one AppID or game must be specified.\n")
			os.Exit(1)
		}

		runtime.GOMAXPROCS(runtime.NumCPU())

		// Create a connection to the master server.
		master, err := valve.NewMasterServerQuerier(config.flag_master)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not query master: %s", err.Error())
		}
		defer master.Close()

		// Set up the filter list.
		master.FilterAppIds(appids)

		// Initialize our batch processor, which will receive servers and query them
		// concurrently.
		bp := batch.NewBatchProcessor(func(bat *batch.BatchProcessor, item interface{}) {
			addr := item.(*net.TCPAddr)

			// if _, ok := item.(valve.ServerList); ok {
			// 	fmt.Println(item.(valve.ServerList).Len())
			// }
			// fmt.Println(remains)
			// if sNumServers > 8 {
			// 	log.Println("*****************&!@(#*!U@OIKJASLKDJAKSLDJLASKD)")
			// 	bat.Terminate()
			// 	// bat.Finish()
			// 	// bat.send_stop
			// }
			flag_timeout := time.Second * 3
			query, err := valve.NewServerQuerier(addr.String(), flag_timeout)
			if err != nil {
				addError(addr.String(), err)
				return
			}
			defer query.Close()

			info, err := query.QueryInfo()
			if err != nil {
				addError(addr.String(), err)
				return
			}

			out := &ServerObject{
				Address: addr.String(),
				// Protocol:   info.Protocol,
				Name:    info.Name,
				MapName: info.MapName,
				// Folder:     info.Folder,
				Game:       info.Game,
				Players:    info.Players,
				MaxPlayers: info.MaxPlayers,
				// Bots:       info.Bots,
				// Type: info.Type.String(),
				// Os:   info.OS.String(),
				// Ship: info.TheShip,
				Mod: info.Mod,
			}
			// if info.Vac == 1 {
			// 	out.Vac = true
			// }
			// if info.Visibility == 0 {
			// 	out.Visibility = "public"
			// } else {
			// 	out.Visibility = "private"
			// }
			// if info.Ext != nil {
			// 	out.AppId = info.Ext.AppId
			// 	out.GameVersion = info.Ext.GameVersion
			// 	out.Port = info.Ext.Port
			// 	out.SteamId = fmt.Sprintf("%d", info.Ext.SteamId)
			// 	out.GameMode = info.Ext.GameModeDescription
			// 	out.GameId = fmt.Sprintf("%d", info.Ext.GameId)
			// }
			// if info.InfoVersion == valve.S2A_INFO_GOLDSRC {
			// 	out.LocalAddress = info.Address
			// }
			// if info.SpecTv != nil {
			// 	out.SpecTvPort = info.SpecTv.Port
			// 	out.SpecTvName = info.SpecTv.Name
			// }

			// We can't query rules for CSGO servers anymore because Valve.
			// csgo := (info.Ext != nil && info.Ext.AppId == valve.App_CSGO)
			// if !csgo && !config.flag_norules {
			// 	log.Println("I', Running this side")
			// 	rules, err := query.QueryRules()
			// 	if err != nil {
			// 		out.Rules = map[string]string{
			// 			"error": err.Error(),
			// 		}
			// 	} else {
			// 		out.Rules = rules
			// 	}
			// }
			out.Rules = map[string]string{"Rules": "NULL"}

			addJson(addr.String(), out, b, m)
		}, config.flag_j)
		//fmt.Println(appids)
		defer bp.Terminate()

		switch sOutputFormat {
		case "list":
			sOutputBuffer.Write([]byte("[\n"))
		case "map":
			sOutputBuffer.Write([]byte("{\n"))
		}

		// Query the master.
		err = master.Query(func(servers valve.ServerList) error {
			bp.AddBatch(servers)
			log.Println("I'm running the main callback")
			// bp.Terminate()
			//bp.Finish()
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not query the master: %s\n", err.Error())
			os.Exit(1)
		}
		//log.Println("NICE")

		// Wait for batch processing to complete.
		bp.Finish()
		var i int = 400
		for i <= len(sResultMap[m.ID]) {
			if _, err := b.Send(m.Sender, sResultMap[m.ID][i-400:i]); err != nil {
				log.Println("ERROR sending text to user cause:", err.Error())
			}
			i += 400
		}
		log.Println("[INFO] All Tasks are done!")
		if sNumServers != 0 {
			sOutputBuffer.Write([]byte("\n"))
		}

		switch sOutputFormat {
		case "list":
			sOutputBuffer.Write([]byte("]\n"))
		case "map":
			sOutputBuffer.Write([]byte("}\n"))
		}

	})
	b.Start()
	// log.Printf("Server does started\n")
}
