package main

import (
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/BurntSushi/toml"
	log "github.com/Sirupsen/logrus"
	"github.com/erictzeng/gobot"
	"github.com/matrix-org/gomatrix"
)

type Config struct {
	Homeserver  string
	UserID      string
	AccessToken string
}

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	var conf Config
	var configData []byte
	var err error
	if configData, err = ioutil.ReadFile("config.toml"); err != nil {
		panic(err)
	}
	if _, err := toml.Decode(string(configData), &conf); err != nil {
		panic(err)
	}

	log.Info("Config: ", conf)
	cli, _ := gomatrix.NewClient(conf.Homeserver, conf.UserID, conf.AccessToken)
	syncer := cli.Syncer.(*gomatrix.DefaultSyncer)

	syncer.OnEventType("m.room.message", func(ev *gomatrix.Event) {
		if ev.Content["body"] == "!ping" {
			cli.SendText(ev.RoomID, "!pong")
		}
	})
	triviaPlugin := gobot.NewTriviaPlugin()
	triviaPlugin.Register(cli)

	for {
		log.Info("Syncing...")
		if err := cli.Sync(); err != nil {
			log.Error("Error: ", err)
		}
	}
}
