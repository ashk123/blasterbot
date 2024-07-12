// vim: set ts=5 sw=4 tw=99 noet:
//
// Blaster (C) Copyright 2014 AlliedModders LLC
// Licensed under the GNU General Public License, version 3 or higher.
// See LICENSE.txt for more details.
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	batch "github.com/alliedmodders/blaster/batch"
	valve "github.com/alliedmodders/blaster/valve"
	"github.com/joho/godotenv"
	"golang.org/x/net/proxy"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	NORM int8 = 3
	ERR  int8 = 1
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
	Visibility string `json:"visibility"`
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

func SendMsg(mode int8, to tb.Recipient, bot *tb.Bot, msg string) {
	var res string = ""
	switch mode {
	case ERR:
		res = "[ERROR] " + msg
	case NORM:
		res = "[INFO] " + msg
		bot.Send(to, res)
	}
}

func Parse(line string) (batch.Config, error) {
	spl_data := strings.Split(line, " ")
	if len(spl_data) <= 1 {
		return batch.Config{}, errors.New(NOT_ENOUGH_ARGS)
	}
	// /search <game> <map> <plen>
	return batch.Config{
		Flag_name:    spl_data[1], // save the first element inside the batch.Config.g_name (the game name)
		Flag_appids:  "",
		Flag_appid:   0,
		Flag_fmap:    spl_data[2],
		Flag_master:  valve.MasterServer,
		Flag_j:       30,
		Flag_norules: false,
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

func mapCompare(r_map string, u_map string) bool {
	if r_map == u_map {
		return true
	}
	return false
}

func addResult(results *map[int]string, key int, value string) {
	sOutputLock.Lock()
	if _, ok := (*results)[key]; ok {
		(*results)[key] += value
	} else {
		(*results)[key] = value
	}
	defer sOutputLock.Unlock()
}

func addJson(hostAndPort string, obj interface{}, bot *tb.Bot, sender *tb.Message, gameFilter *batch.Config) {
	if _, err := obj.(*ErrorObject); err { // If we have error, don't even process it
		return
	}
	var isFilterTrue bool = false
	log.Println("[INFO] Processing Game Server ...")
	// If we had any problem with tel objects
	if bot == nil || sender == nil {
		return
	}
	serv_obj_org, res := obj.(*ServerObject)
	if res {

	}
	if mapCompare(serv_obj_org.MapName, gameFilter.Flag_fmap) {
		isFilterTrue = true
	} else {
		isFilterTrue = false
	}

	if isFilterTrue {
		addResult(&sResultMap, sender.ID, fmt.Sprintf("Name: %s\nPlayers: %d\nMap: %s\n------------\n", serv_obj_org.Name, serv_obj_org.Players, serv_obj_org.MapName))
	}
	log.Println("[INFO] Processing Game Server Succsessfully")
}

func addError(hostAndPort string, err error) {
	addJson(hostAndPort, &ErrorObject{
		Ip:    hostAndPort,
		Error: err.Error(),
	}, nil, nil, nil)
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("ERROR: Can't read the env file for reason:", err.Error())
	}
	token := os.Getenv("BOT_TOKEN")
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
			log.Println("ERROR:", err.Error())
		}
		config, err := Parse(m.Text)
		fmt.Println("Telegram Arguments Parsed")
		if err != nil {
			b.Send(m.Sender, err)
			return
		}
		sOutputFormat = config.Flag_outfile

		// Flag_game := flag.String("game", "", "Game (hl1, hl2)")
		// Flag_appid := flag.Int("appid", 0, "Query a single AppID")
		// Flag_appids := flag.String("appids", "", "Comma-delimited list of AppIDs")
		// Flag_master := flag.String("master", valve.MasterServer, "Master server address")
		// Flag_j := flag.Int("j", 20, "Number of concurrent requests (more will introduce more timeouts)")
		// Flag_timeout := flag.Duration("timeout", time.Second*3, "Timeout for querying servers")
		// Flag_format := flag.String("format", "list", "JSON format (list, map, or lines)")
		// Flag_outfile := flag.String("outfile", "", "Output to a file")
		// Flag_norules := flag.Bool("norules", false, "Don't query server rules")
		// Flag.Usage = func() {
		// 	fmt.Fprintf(os.Stderr, "Usage: -game or -appids\n")
		// 	Flag.PrintDefaults()
		// }
		// Flag.Parse()

		appids := []valve.AppId{}
		//fmt.Printf("This is the game name: |%s|\n", config.Flag_name)
		if config.Flag_name != "" {
			switch config.Flag_name {
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
				SendMsg(NORM, m.Sender, b,
					fmt.Sprintf("Unrecognized game: %s", config.Flag_name))
				return
			}
		}

		if len(appids) == 0 {
			fmt.Fprintf(os.Stderr, "At least one AppID or game must be specified.\n")
			os.Exit(1)
		}

		runtime.GOMAXPROCS(runtime.NumCPU())

		// Create a connection to the master server.
		master, err := valve.NewMasterServerQuerier(config.Flag_master)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not query master: %s", err.Error())
		}
		defer master.Close()

		// Set up the filter list.
		master.FilterAppIds(appids)

		bp := batch.NewBatchProcessor(func(gameFilter *batch.Config, item interface{}) {
			addr := item.(*net.TCPAddr)

			Flag_timeout := time.Second * 3
			query, err := valve.NewServerQuerier(addr.String(), Flag_timeout)
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
				//Bots:       info.Bots,
				// Type: info.Type.String(),
				// Os:   info.OS.String(),
				// Ship: info.TheShip,
				Mod: info.Mod,
			}
			// if info.Vac == 1 {
			// 	out.Vac = true
			// }
			if info.Visibility == 0 {
				out.Visibility = "public"
			} else {
				out.Visibility = "private"
			}
			//if info.Ext != nil {
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
			// if !csgo && !config.Flag_norules {
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

			addJson(addr.String(), out, b, m, gameFilter)
		}, config.Flag_j)
		bp.SetConfig(config) // set the user config as gameFilter
		defer bp.Terminate()

		// Query the master.
		err = master.Query(func(servers valve.ServerList) error {
			bp.AddBatch(servers)
			//bp.Finish()
			return nil
		})
		if err != nil {
			SendMsg(ERR, m.Sender, b, "There is a error when query the valve's servers")
			return
		}
		// Wait for batch processing to complete.
		bp.Finish()
		if _, err := b.Send(m.Sender, sResultMap[m.ID][:400]); err != nil {
			log.Println("ERROR sending text to user cause:", err.Error())
		}
		log.Println("[INFO] All Tasks are done!")
		SendMsg(NORM, m.Sender, b, "That's it, Those are your desire results :D")
	})
	b.Start()
	// log.Printf("Server does started\n")
}
