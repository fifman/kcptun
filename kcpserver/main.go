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
	myApp.Usage = "server(with SMUX)"
	myApp.Version = src.VERSION
	myApp.Flags = src.BuildServerFlags()
	myApp.Action = func(c *cli.Context) {
		config := src.ServerConfig{}
		src.SetConfig(&config.Config, c)
		config.Pprof = c.Bool("pprof")

		src.ParseConfigFile(&config, c)
		src.PostLoadConfig(&config.Config)

		src.RunServer(config)
	}
	myApp.Run(os.Args)
}
