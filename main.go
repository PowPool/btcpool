// +build go1.9

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	//"github.com/yvasiyarov/gorelic"

	"github.com/MiningPool0826/btcpool/api"
	"github.com/MiningPool0826/btcpool/payouts"
	"github.com/MiningPool0826/btcpool/proxy"
	"github.com/MiningPool0826/btcpool/storage"
	. "github.com/MiningPool0826/btcpool/util"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	LatestTag           = ""
	LatestTagCommitSHA1 = ""
	LatestCommitSHA1    = ""
	BuildTime           = ""
)

var cfg proxy.Config
var backend *storage.RedisClient

func startProxy() {
	s := proxy.NewProxy(&cfg, backend)
	s.Start()
}

func startApi() {
	s := api.NewApiServer(&cfg.Api, backend)
	s.Start()
}

func startBlockUnlocker() {
	u := payouts.NewBlockUnlocker(&cfg.BlockUnlocker, backend)
	u.Start()
}

//func startPayoutsProcessor() {
//	u := payouts.NewPayoutsProcessor(&cfg.Payouts, backend)
//	u.Start()
//}

// this function is for performance profile
//func startNewrelic() {
//	if cfg.NewrelicEnabled {
//		nr := gorelic.NewAgent()
//		nr.Verbose = cfg.NewrelicVerbose
//		nr.NewrelicLicense = cfg.NewrelicKey
//		nr.NewrelicName = cfg.NewrelicName
//		nr.Run()
//	}
//}

func readConfig(cfg *proxy.Config) {
	configFileName := "config.json"
	if len(os.Args) > 1 {
		configFileName = os.Args[1]
	}
	configFileName, _ = filepath.Abs(configFileName)
	log.Printf("Loading config: %v", configFileName)

	configFile, err := os.Open(configFileName)
	if err != nil {
		log.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&cfg); err != nil {
		log.Fatal("Config error: ", err.Error())
	}
}

func readSecurityPass() ([]byte, error) {
	fmt.Printf("Enter Security Password: ")
	var fd int
	if terminal.IsTerminal(int(syscall.Stdin)) {
		fd = int(syscall.Stdin)
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return nil, errors.New("error allocating terminal")
		}
		defer tty.Close()
		fd = int(tty.Fd())
	}

	SecurityPass, err := terminal.ReadPassword(fd)
	if err != nil {
		return nil, err
	}
	return SecurityPass, nil
}

func decryptPoolConfigure(cfg *proxy.Config, passBytes []byte) error {
	b, err := Ae64Decode(cfg.UpstreamCoinBaseEncrypted, passBytes)
	if err != nil {
		return err
	}
	cfg.UpstreamCoinBase = string(b)
	// check address
	if !IsValidBTCAddress(cfg.UpstreamCoinBase) {
		return errors.New("decryptPoolConfigure: IsValidBTCAddress")
	}

	b, err = Ae64Decode(cfg.Redis.PasswordEncrypted, passBytes)
	if err != nil {
		return err
	}
	cfg.Redis.Password = string(b)

	return nil
}

func getDeviceIPs() (map[string]struct{}, error) {
	ipAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	DeviceIPs := make(map[string]struct{})
	for _, addr := range ipAddrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				DeviceIPs[ip4.String()] = struct{}{}
			}
		}
	}
	return DeviceIPs, nil
}

func initPeerName(cfg *proxy.Config) error {
	deviceIPs, err := getDeviceIPs()
	if err != nil {
		return err
	}

	for _, c := range cfg.Cluster {
		_, ok := deviceIPs[c.NodeIp]
		if ok {
			cfg.Name = c.NodeName
			cfg.Id = c.NodeId
			return nil
		}
	}
	return errors.New("local Node is not in the Pool cluster")
}

func OptionParse() {
	var showVer bool
	flag.BoolVar(&showVer, "v", false, "show build version")

	flag.Parse()

	if showVer {
		fmt.Printf("Latest Tag: %s\n", LatestTag)
		fmt.Printf("Latest Tag Commit SHA1: %s\n", LatestTagCommitSHA1)
		fmt.Printf("Latest Commit SHA1: %s\n", LatestCommitSHA1)
		fmt.Printf("Build Time: %s\n", BuildTime)
		os.Exit(0)
	}
}

func main() {
	OptionParse()
	readConfig(&cfg)
	//rand.Seed(time.Now().UnixNano())

	// init log file
	_ = os.Mkdir("logs", os.ModePerm)
	iLogFile := "logs/info.log"
	eLogFile := "logs/error.log"
	sLogFile := "logs/share.log"
	bLogFile := "logs/block.log"
	InitLog(iLogFile, eLogFile, sLogFile, bLogFile, cfg.Log.LogSetLevel)

	err := initPeerName(&cfg)
	if err != nil {
		Error.Fatal("initPeerName error: ", err.Error())
	}
	Info.Println("Init Peer Name as:", cfg.Name)

	if cfg.Threads > 0 {
		runtime.GOMAXPROCS(cfg.Threads)
		Info.Printf("Running with %v threads", cfg.Threads)
	}

	//startNewrelic()

	secPassBytes, err := readSecurityPass()
	if err != nil {
		Error.Fatal("Read Security Password error: ", err.Error())
	}

	err = decryptPoolConfigure(&cfg, secPassBytes)
	if err != nil {
		Error.Fatal("Decrypt Pool Configure error: ", err.Error())
	}

	backend = storage.NewRedisClient(&cfg.Redis, cfg.Coin)
	pong, err := backend.Check()
	if err != nil {
		Error.Printf("Can't establish connection to backend: %v", err)
	} else {
		Error.Printf("Backend check reply: %v", pong)
	}

	defer func() {
		if r := recover(); r != nil {
			Error.Println(string(debug.Stack()))
		}
	}()

	if cfg.Proxy.Enabled {
		go startProxy()
	}
	if cfg.Api.Enabled {
		go startApi()
	}
	if cfg.BlockUnlocker.Enabled {
		go startBlockUnlocker()
	}
	//if cfg.Payouts.Enabled {
	//	go startPayoutsProcessor()
	//}
	quit := make(chan bool)
	<-quit
}
