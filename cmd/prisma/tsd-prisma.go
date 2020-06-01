package main

import (
	"context"
	"errors"
	"fmt"
	slog "log"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/egorka-gh/sm/prisma"
	"github.com/egorka-gh/sm/tsd"
	"github.com/kardianos/osext"

	"github.com/kardianos/service"
	group "github.com/oklog/oklog/pkg/group"

	"github.com/spf13/viper"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	log "github.com/go-kit/kit/log"
	_ "github.com/godror/godror"
)

func main() {
	err := readConfig()
	if err != nil {
		slog.Print(err) // .Fatal(err)
		//return
		//slog.Print("Start using default config")
		viper.SetConfigFile("config.json")
		err = viper.SafeWriteConfig()
		if err != nil {
			slog.Print(err)
			return
		}
		slog.Print("Default config file created, exiting...")
		//return
	}

	svcConfig := &service.Config{
		Name:        "TSD-PRISMA",
		DisplayName: "TSD-PRISMA",
		Description: "Transeferring SM2000 TSD events to PRISMA",
	}
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		slog.Fatal(err)
		return
	}
	if len(os.Args) > 1 {
		err = service.Control(s, os.Args[1])
		if err != nil {
			slog.Fatal(err)
		}
		return
	}
	dLogger, err = s.Logger(nil)
	if err != nil {
		slog.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		dLogger.Error(err)
	}
}

//demon logger
var dLogger service.Logger

//
type program struct {
	group     *group.Group
	interrupt chan struct{}
	quit      chan struct{}
}

func (p *program) Start(s service.Service) error {
	g, err := initPrisma()
	if err != nil {
		return err
	}

	p.group = g
	p.interrupt = make(chan struct{})
	p.quit = make(chan struct{})

	if service.Interactive() {
		dLogger.Info("Running in terminal.")
		dLogger.Infof("Valid startup parametrs: %q\n", service.ControlAction)
	} else {
		dLogger.Info("Starting service...")
	}
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

func (p *program) run() {
	running := make(chan struct{})
	//initCancelInterrupt
	p.group.Add(
		func() error {
			select {
			case <-p.interrupt:
				return errors.New("Get interrupt signal")
			case <-running:
				return nil
			}
		}, func(error) {
			close(running)
		})
	dLogger.Info("Started")
	dLogger.Info(p.group.Run())
	close(p.quit)
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	dLogger.Info("Stopping!")
	//interrupt service
	close(p.interrupt)
	//waite service stops
	<-p.quit
	dLogger.Info("Stopped")
	return nil
}

func initPrisma() (*group.Group, error) {
	if viper.GetString("oracle") == "" {
		return nil, errors.New("Не задано подключение oracle")
	}
	if viper.GetString("prisma.host") == "" {
		return nil, errors.New("Не задан адрес призмы")
	}
	repo, err := tsd.NewRepo(viper.GetString("oracle"))
	if err != nil {
		return nil, err
	}

	p := &smprisma{
		logger:   initLoger(viper.GetString("log_folder"), "prisma"),
		cashNum:  viper.GetInt("prisma.cash"),
		locid:    viper.GetInt("locid"),
		interval: viper.GetInt("interval"),
		lastsync: time.Now(),
		repo:     repo,
		client:   prisma.NewClient(viper.GetString("prisma.host")),
	}
	if p.interval < 3 {
		p.interval = 3
	}
	//TODO log config
	p.logger.Log("event", "worker created")

	g := &group.Group{}
	//main worker
	quit := make(chan struct{})
	g.Add(func() error {
		p.logger.Log("event", "worker started")
		//var cancel context.CancelFunc
		isRunning := true
		for isRunning {
			select {
			case <-time.After(time.Duration(p.interval) * time.Minute):
				//ctx, cancel := context.WithCancel(context.Background())
				p.sync(context.Background())
			case <-quit:
				isRunning = false
			}
		}
		p.repo.Close()
		return nil
	}, func(error) {
		close(quit)
	})
	return g, nil
}

type smprisma struct {
	repo     tsd.Repository
	client   *prisma.Client
	logger   log.Logger
	lastsync time.Time
	locid    int
	cashNum  int
	interval int
}

func (s *smprisma) sync(ctx context.Context) {
	s.logger.Log("event", "start", "sync", s.lastsync.Format("2006-01-02 15:04:05"))
	//get docs
	docs, err := s.repo.LoadDocByTime(ctx, s.lastsync, s.locid)
	if err != nil {
		s.logger.Log("error", err)
		s.logger.Log("event", "end")
		return
	}
	l := len(docs)
	s.logger.Log("doc.count", l)
	if l == 0 {
		s.logger.Log("event", "end")
		return
	}
	ids := make([]string, 0, l)
	lastsync := s.lastsync
	for i := range docs {
		ids = append(ids, docs[i].ProcessID)
		if docs[i].DBTime.After(lastsync) {
			lastsync = docs[i].DBTime
		}
	}
	s.logger.Log("doc.ids", strings.Join(ids, ","))
	//create message batch
	messages := make([]*prisma.Message, 0)
	for _, d := range docs {
		messages = append(messages, doc2messagess(d, s.cashNum)...)
	}
	s.logger.Log("messages.count", len(messages))
	//TODO cancel by context??
	err = s.client.SendBatch(messages)
	if err != nil {
		s.logger.Log("error", err)
	}
	s.lastsync = lastsync
	s.logger.Log("event", "end", "nextsync", s.lastsync.Format("2006-01-02 15:04:05"))
}

func doc2messagess(d tsd.Document, cashNum int) []*prisma.Message {
	messages := make([]*prisma.Message, 0, len(d.Items)+2)
	//check open
	m := prisma.Message{
		Prefix:     "KKM",
		Number:     cashNum,
		Mode:       4,
		CassirItem: fmt.Sprintf("%d", d.UserID),
		Cassir:     prisma.Pstring(d.UserName),
		CKNumber:   d.ProcessID,
		Count:      0,
		BarCode:    "",
		GoodsItem:  "",
		GoodsName:  prisma.Pstring(fmt.Sprintf("Заказ %s", d.BaseDoc)),
		GoodsQuant: 0,
		Year:       d.StartTime.Format("06"),
		Month:      d.StartTime.Format("01"),
		Day:        d.StartTime.Format("02"),
		Hour:       d.StartTime.Format("15"),
		Min:        d.StartTime.Format("04"),
		Sec:        d.StartTime.Format("05"),
		Sec100:     "00",
	}
	messages = append(messages, &m)

	//doc items
	//item := d.Items[0]
	maxitem := 1
	for _, item := range d.Items {
		mi := m
		mi.Mode = 6
		mi.Count = item.ItemID
		if maxitem < item.ItemID {
			maxitem = item.ItemID
		}
		mi.BarCode = item.Barcode
		mi.GoodsItem = prisma.Pstring(item.Article)
		mi.GoodsName = prisma.Pstring(item.CardName)
		mi.GoodsPrice = math.Round(item.Pack*1000) / 1000
		mi.GoodsQuant = math.Round(item.QttPack*1000) / 1000
		mi.GoodsSum = math.Round(item.Qtt*1000) / 1000
		mi.Year = item.EventTime.Format("06")
		mi.Month = item.EventTime.Format("01")
		mi.Day = item.EventTime.Format("02")
		mi.Hour = item.EventTime.Format("15")
		mi.Min = item.EventTime.Format("04")
		mi.Sec = item.EventTime.Format("05")
		messages = append(messages, &mi)
	}

	//finalize
	me := m
	me.Mode = 5
	maxitem++
	me.Count = maxitem
	me.BarCode = ""
	me.GoodsItem = ""
	me.GoodsName = prisma.Pstring(fmt.Sprintf("Накладная %s", d.ResultDoc))
	me.GoodsPrice = 0
	me.GoodsQuant = 0
	me.GoodsSum = 0
	me.Year = d.EndTime.Format("06")
	me.Month = d.EndTime.Format("01")
	me.Day = d.EndTime.Format("02")
	me.Hour = d.EndTime.Format("15")
	me.Min = d.EndTime.Format("04")
	me.Sec = d.EndTime.Format("05")
	messages = append(messages, &me)
	return messages
}

func initLoger(logPath, fileName string) log.Logger {
	var logger log.Logger
	if logPath == "" {
		logger = log.NewLogfmtLogger(os.Stderr)
	} else {
		if fileName == "" {
			fileName = "log"
		}
		p := path.Join(logPath, fmt.Sprintf("%s.log", fileName))
		logger = log.NewLogfmtLogger(&lumberjack.Logger{
			Filename:   p,
			MaxSize:    5, // megabytes
			MaxBackups: 5,
			MaxAge:     60, //days
		})
	}
	logger = log.With(logger, "ts", log.DefaultTimestamp) // .DefaultTimestampUTC)
	logger = log.With(logger, "caller", log.DefaultCaller)

	return logger
}

func readConfig() error {
	viper.SetDefault("oracle", "supermag/fhfvbc1999@skont08") //oracle connection string
	viper.SetDefault("prisma.host", "192.168.29.9")           //prisma host
	viper.SetDefault("prisma.cash", "1")                      //prisma cash desk number
	viper.SetDefault("locid", -1)                             //sm storelocation (-1 all)
	viper.SetDefault("interval", 3)                           //processing interval (min)
	viper.SetDefault("log_folder", ".\\log")                  //Log folder

	path, err := osext.ExecutableFolder()
	if err != nil {
		path = "."
	}
	//fmt.Println("Path ", path)
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	return viper.ReadInConfig()
}
