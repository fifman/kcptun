package main

import (
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/urfave/cli"
	"github.com/fifman/kcptun/src"
)

func main() {
	rand.Seed(int64(time.Now().Nanosecond()))
	if src.VERSION == "SELFBUILD" {
		// add more log flags for debugging
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
	myApp := cli.NewApp()
	myApp.Name = "kcptun"
	myApp.Usage = "client(with SMUX)"
	myApp.Version = src.VERSION
	myApp.Flags = src.BuildClientFlags()
	myApp.Action = func(c *cli.Context) {
		config := src.ClientConfig{}

		src.SetConfig(&config.Config, c)

		config.Conn = c.Int("conn")
		config.AutoExpire = c.Int("autoexpire")
		config.ScavengeTTL = c.Int("scavengettl")

		src.ParseConfigFile(&config, c)
		src.PostLoadConfig(&config.Config)

		src.RunClient(config)
	}
	myApp.Run(os.Args)
}
